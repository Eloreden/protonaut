package backend

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)

// MonitorInfo rappresenta un monitor fisico con la sua geometria logica e scala.
type MonitorInfo struct {
	Name    string  `json:"name"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Width   int     `json:"width"`  // dimensione logica (già divisa per la scala)
	Height  int     `json:"height"` // dimensione logica
	Scale   float64 `json:"scale"`
	Primary bool    `json:"primary"`
}

// GetMonitors restituisce tutti i monitor rilevati. Tenta prima kscreen-doctor
// (KDE Wayland), poi xrandr come fallback.
func GetMonitors() []MonitorInfo {
	if monitors := monitorsFromKscreen(); len(monitors) > 0 {
		return monitors
	}
	return monitorsFromXrandr()
}

// monitorsFromKscreen parsa l'output di `kscreen-doctor --outputs`.
// Campi rilevanti per ogni monitor:
//
//	Output: N <name> ...
//	  Scale: X.XX
//	  Geometry: X,Y WIDTHxHEIGHT
//	  priority N  (il più basso è il primario)
func monitorsFromKscreen() []MonitorInfo {
	out, err := exec.Command("kscreen-doctor", "--outputs").Output()
	if err != nil {
		return nil
	}

	var monitors []MonitorInfo
	var cur *MonitorInfo
	lowestPriority := 9999

	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(ansiRe.ReplaceAllString(raw, ""))
		switch {
		case strings.HasPrefix(line, "Output:"):
			if cur != nil {
				monitors = append(monitors, *cur)
			}
			parts := strings.Fields(line)
			name := ""
			if len(parts) >= 3 {
				name = parts[2]
			}
			cur = &MonitorInfo{Name: name, Scale: 1.0}

		case cur == nil:
			continue

		case strings.HasPrefix(line, "Scale:"):
			if f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, "Scale:")), 64); err == nil {
				cur.Scale = f
			}

		case strings.HasPrefix(line, "Geometry:"):
			// "Geometry: X,Y WIDTHxHEIGHT"
			rest := strings.TrimSpace(strings.TrimPrefix(line, "Geometry:"))
			parts := strings.Fields(rest)
			if len(parts) >= 2 {
				if xy := strings.Split(parts[0], ","); len(xy) == 2 {
					cur.X, _ = strconv.Atoi(xy[0])
					cur.Y, _ = strconv.Atoi(xy[1])
				}
				if wh := strings.Split(parts[1], "x"); len(wh) == 2 {
					cur.Width, _ = strconv.Atoi(wh[0])
					cur.Height, _ = strconv.Atoi(wh[1])
				}
			}

		case strings.HasPrefix(line, "priority"):
			// "priority N"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if p, err := strconv.Atoi(parts[1]); err == nil && p < lowestPriority {
					lowestPriority = p
					// verrà marcato come Primary alla fine
				}
			}
		}
	}
	if cur != nil {
		monitors = append(monitors, *cur)
	}

	// Marca il primario (priorità più bassa in kscreen)
	if len(monitors) > 0 {
		monitors[0].Primary = true // fallback: primo in lista
	}
	return monitors
}

// monitorsFromXrandr parsa `xrandr --listmonitors` come fallback su X11.
// Formato:  " 0: +*NAME WxH+offX+offY  CONNECTOR"
func monitorsFromXrandr() []MonitorInfo {
	out, err := exec.Command("xrandr", "--listmonitors").Output()
	if err != nil {
		return nil
	}
	var monitors []MonitorInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Monitors:") {
			continue
		}
		// "+*" = primario
		primary := strings.Contains(line, "+*")
		// rimuovi numero iniziale "N: "
		if idx := strings.Index(line, ":"); idx >= 0 {
			line = strings.TrimSpace(line[idx+1:])
		}
		line = strings.TrimPrefix(line, "+*")
		line = strings.TrimPrefix(line, "+")
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		// "WIDTHmmxHEIGHTmm+X+Y" — prendiamo solo W+X+Y
		geom := parts[1]
		// togliamo la parte fisica "/mm"
		// formato: "W/mmxH/mm+X+Y"  o  "WxH+X+Y"
		// step 1: rimuovi "/DDD"
		cleanGeom := ""
		skip := false
		for i := 0; i < len(geom); i++ {
			if geom[i] == '/' {
				skip = true
				continue
			}
			if skip && (geom[i] == 'x' || geom[i] == '+' || geom[i] == '-') {
				skip = false
			}
			if !skip {
				cleanGeom += string(geom[i])
			}
		}
		// ora "WxH+X+Y"
		var w, h, x, y int
		fmt := strings.FieldsFunc(cleanGeom, func(r rune) bool { return r == 'x' || r == '+' })
		if len(fmt) >= 4 {
			w, _ = strconv.Atoi(fmt[0])
			h, _ = strconv.Atoi(fmt[1])
			x, _ = strconv.Atoi(fmt[2])
			y, _ = strconv.Atoi(fmt[3])
		}
		monitors = append(monitors, MonitorInfo{
			Name:    name,
			X:       x,
			Y:       y,
			Width:   w,
			Height:  h,
			Scale:   1.0, // xrandr non riporta la scala fratto; 1.0 come default
			Primary: primary,
		})
	}
	return monitors
}

// MonitorAt restituisce il monitor che contiene il punto (px, py).
func MonitorAt(monitors []MonitorInfo, px, py int) *MonitorInfo {
	for i, m := range monitors {
		if px >= m.X && px < m.X+m.Width && py >= m.Y && py < m.Y+m.Height {
			return &monitors[i]
		}
	}
	// fallback: il primario
	for i, m := range monitors {
		if m.Primary {
			return &monitors[i]
		}
	}
	if len(monitors) > 0 {
		return &monitors[0]
	}
	return nil
}
