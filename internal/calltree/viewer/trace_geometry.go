package viewer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Geometry is data only: the browser draws it on a bounded viewport canvas.
// The Graphviz SVG remains an isolated image, never executable page markup.
type graphTraces struct {
	Transform [4]float64   `json:"transform"` // sx, sy, tx, ty: Graphviz points -> image pixels
	Sites     []graphSite  `json:"sites"`
	Routes    []graphRoute `json:"routes"`
}

type graphSite struct {
	From   int    `json:"from"`
	Site   string `json:"site"`
	Label  string `json:"label"`
	Bounds [4]int `json:"bounds"`
}

type graphRoute struct {
	Kind         string       `json:"kind,omitempty"`
	Registration *int         `json:"registration,omitempty"`
	Label        string       `json:"label,omitempty"`
	From         int          `json:"from"`
	To           int          `json:"to"`
	Site         string       `json:"site"`
	Path         string       `json:"path"`
	Head         [][2]float64 `json:"head,omitempty"`
	Origin       [3]float64   `json:"origin"`
	Color        string       `json:"color"`
	Dash         []float64    `json:"dash,omitempty"`
}

var tracePath = regexp.MustCompile(`^[MCmc0-9eE.,+\s-]+$`)
var traceColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func traceIdentity(href, prefix string, count int) ([]string, error) {
	parts := strings.SplitN(strings.TrimPrefix(href, prefix), ":", count)
	if len(parts) != count {
		return nil, fmt.Errorf("invalid trace identity: %q", href)
	}
	for _, part := range parts[:count-1] {
		id, err := strconv.Atoi(part)
		if err != nil || id < 0 {
			return nil, fmt.Errorf("invalid trace identity: %q", href)
		}
	}
	return parts, nil
}

func traceNumbers(value string) ([]float64, error) {
	var numbers []float64
	for _, field := range strings.Fields(strings.ReplaceAll(value, ",", " ")) {
		number, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("invalid trace coordinate: %q", field)
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

func readTraceGeometry(svg []byte) (graphTraces, error) {
	traces := graphTraces{Sites: []graphSite{}, Routes: []graphRoute{}}
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	depth, linkDepth, route := 0, 0, -1
	byLink := make(map[string]int)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return traces, nil
		}
		if err != nil {
			return traces, fmt.Errorf("decode graph trace geometry: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			depth++
			attributes := make(map[string]string, len(token.Attr))
			for _, attribute := range token.Attr {
				attributes[attribute.Name.Local] = attribute.Value
			}
			if token.Name.Local == "g" && attributes["class"] == "graph" {
				var sx, sy, rotation, tx, ty float64
				n, err := fmt.Sscanf(attributes["transform"], "scale(%f %f) rotate(%f) translate(%f %f)", &sx, &sy, &rotation, &tx, &ty)
				if err != nil || n != 5 || rotation != 0 || sx <= 0 || sy <= 0 {
					return traces, fmt.Errorf("unsupported graph trace transform")
				}
				traces.Transform = [4]float64{sx * 4 / 3, sy * 4 / 3, tx * sx * 4 / 3, ty * sy * 4 / 3}
				for _, value := range traces.Transform {
					if math.IsNaN(value) || math.IsInf(value, 0) {
						return traces, fmt.Errorf("invalid graph trace transform")
					}
				}
			}
			registration := strings.HasPrefix(attributes["href"], "registration:")
			if token.Name.Local == "a" && (strings.HasPrefix(attributes["href"], "call:") || registration) {
				href := attributes["href"]
				prefix, count := "call:", 3
				if registration {
					prefix, count = "registration:", 4
				}
				parts, err := traceIdentity(href, prefix, count)
				if err != nil {
					return traces, err
				}
				var exists bool
				route, exists = byLink[href]
				if !exists {
					route = len(traces.Routes)
					byLink[href] = route
					from, _ := strconv.Atoi(parts[0])
					to, _ := strconv.Atoi(parts[1])
					geometry := graphRoute{From: from, To: to, Site: parts[len(parts)-1]}
					if registration {
						id, _ := strconv.Atoi(parts[2])
						geometry.Kind, geometry.Registration = "registration", &id
						geometry.Label = strings.SplitN(attributes["title"], "\n", 2)[0]
					}
					traces.Routes = append(traces.Routes, geometry)
				}
				linkDepth = depth
			}
			if route < 0 {
				continue
			}
			current := &traces.Routes[route]
			switch token.Name.Local {
			case "path":
				if !tracePath.MatchString(attributes["d"]) {
					return traces, fmt.Errorf("invalid graph trace path")
				}
				coordinates := strings.NewReplacer("M", " ", "C", " ", "m", " ", "c", " ").Replace(attributes["d"])
				positions, parseErr := traceNumbers(coordinates)
				if parseErr != nil {
					return traces, parseErr
				}
				if len(positions) < 8 || (len(positions)-2)%6 != 0 {
					return traces, fmt.Errorf("invalid graph trace spline")
				}
				current.Path = attributes["d"]
				current.Color = attributes["stroke"]
				if !traceColor.MatchString(current.Color) {
					return traces, fmt.Errorf("invalid graph trace color")
				}
				current.Dash, err = traceNumbers(attributes["stroke-dasharray"])
			case "polygon":
				var points []float64
				points, err = traceNumbers(attributes["points"])
				if len(points)%2 != 0 {
					return traces, fmt.Errorf("invalid graph trace arrowhead")
				}
				for i := 0; i < len(points); i += 2 {
					current.Head = append(current.Head, [2]float64{points[i], points[i+1]})
				}
			case "ellipse":
				for i, attribute := range []string{"cx", "cy", "rx"} {
					var values []float64
					values, err = traceNumbers(attributes[attribute])
					if err != nil || len(values) != 1 {
						return traces, fmt.Errorf("invalid graph trace origin")
					}
					current.Origin[i] = values[0]
				}
				if current.Origin[2] < 0 {
					return traces, fmt.Errorf("invalid graph trace origin radius")
				}
			}
			if err != nil {
				return traces, err
			}
		case xml.EndElement:
			if depth == linkDepth {
				route, linkDepth = -1, 0
			}
			depth--
		}
	}
}
