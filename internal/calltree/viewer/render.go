// Package viewer renders and serves call graphs without depending on Cobra.
package viewer

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

const (
	renderTimeout      = 30 * time.Second
	maxDOTBytes        = 8 << 20
	maxSVGBytes        = 32 << 20
	maxDiagnosticBytes = 64 << 10
)

var errOutputLimit = errors.New("graph rendering output exceeds size limit")

// CheckGraphviz checks the optional browser-view dependency before analysis.
func CheckGraphviz() error {
	if _, err := exec.LookPath("dot"); err != nil {
		return errors.New("Graphviz (dot) is required for browser view; install Graphviz or use --format dot/json")
	}
	return nil
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *limitedBuffer) String() string { return b.buffer.String() }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if len(data) > remaining {
		n, _ := b.buffer.Write(data[:remaining])
		return n, errOutputLimit
	}
	return b.buffer.Write(data)
}

// RenderSVG requires Graphviz's dot executable. DOT/JSON exports do not.
func RenderSVG(ctx context.Context, graph *model.Graph) ([]byte, error) {
	return renderViewSVG(ctx, diagram.IndexGraph(graph), diagram.BrowserOptions{Focus: -1})
}

func renderViewSVG(ctx context.Context, graph *diagram.IndexedGraph, options diagram.BrowserOptions) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()
	dot := limitedBuffer{limit: maxDOTBytes}
	svg := limitedBuffer{limit: maxSVGBytes}
	stderr := limitedBuffer{limit: maxDiagnosticBytes}
	if err := graph.WriteBrowserDOT(&dot, options); err != nil {
		return nil, fmt.Errorf("encode DOT: %w", err)
	}
	// Graphviz emits the SVG followed by its matching pixel-coordinate image map.
	// Only rectangles and numeric function IDs are retained from that map.
	command := exec.CommandContext(ctx, "dot", "-Tsvg", "-Tcmapx")
	command.Stdin = bytes.NewReader(dot.Bytes())
	command.Stdout = &svg
	command.Stderr = &stderr
	// Bound waiting for inherited pipes if a renderer child outlives dot.
	command.WaitDelay = 2 * time.Second
	err := command.Run()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("render graph with Graphviz (install dot or use --format dot/json): %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	image, err := embedNodeMap(svg.Bytes())
	if err != nil {
		return nil, err
	}
	return separateEdgeStrokes(image)
}

type graphNode struct {
	ID     int    `json:"id"`
	Label  string `json:"label"`
	Bounds [4]int `json:"bounds"`
}

func embedNodeMap(output []byte) ([]byte, error) {
	end := bytes.LastIndex(output, []byte("</svg>"))
	if end < 0 {
		return nil, errors.New("Graphviz returned an invalid SVG")
	}
	var imageMap struct {
		Areas []struct {
			Shape  string `xml:"shape,attr"`
			Href   string `xml:"href,attr"`
			Title  string `xml:"title,attr"`
			Coords string `xml:"coords,attr"`
		} `xml:"area"`
	}
	mapXML := bytes.TrimSpace(output[end+len("</svg>"):])
	// Some renderers/test stubs provide only SVG; it remains usable without hits.
	if len(mapXML) == 0 {
		return output, nil
	}
	if err := xml.Unmarshal(mapXML, &imageMap); err != nil {
		return nil, fmt.Errorf("decode Graphviz node map: %w", err)
	}
	nodes := []graphNode{}
	traces, err := readTraceGeometry(output[:end+len("</svg>")])
	if err != nil {
		return nil, err
	}
	for _, area := range imageMap.Areas {
		isSite := strings.HasPrefix(area.Href, "site:")
		if area.Shape != "rect" || (!strings.HasPrefix(area.Href, "function:") && !isSite) {
			continue
		}
		identity := strings.TrimPrefix(area.Href, "function:")
		var site string
		if isSite {
			parts, err := traceIdentity(area.Href, "site:", 2)
			if err != nil {
				return nil, err
			}
			identity, site = parts[0], parts[1]
		}
		id, err := strconv.Atoi(identity)
		if err != nil || id < 0 {
			return nil, errors.New("invalid function ID in Graphviz node map")
		}
		coords := strings.Split(area.Coords, ",")
		if len(coords) != 4 {
			return nil, errors.New("invalid rectangle in Graphviz node map")
		}
		node := graphNode{ID: id, Label: area.Title}
		for index, coordinate := range coords {
			node.Bounds[index], err = strconv.Atoi(strings.TrimSpace(coordinate))
			if err != nil || node.Bounds[index] < 0 {
				return nil, errors.New("invalid coordinates in Graphviz node map")
			}
		}
		if node.Bounds[2] <= node.Bounds[0] || node.Bounds[3] <= node.Bounds[1] {
			return nil, errors.New("empty rectangle in Graphviz node map")
		}
		if isSite {
			traces.Sites = append(traces.Sites, graphSite{From: id, Site: site, Label: area.Title, Bounds: node.Bounds})
		} else {
			nodes = append(nodes, node)
		}
	}
	encoded, err := json.Marshal(nodes)
	if err != nil {
		return nil, err
	}
	var svg bytes.Buffer
	svg.Write(output[:end])
	svg.WriteString(`<metadata id="codebound-nodes">`)
	if err := xml.EscapeText(&svg, encoded); err != nil {
		return nil, err
	}
	svg.WriteString("</metadata>")
	if len(traces.Routes) > 0 {
		encoded, err = json.Marshal(traces)
		if err != nil {
			return nil, err
		}
		svg.WriteString(`<metadata id="codebound-traces">`)
		if err := xml.EscapeText(&svg, encoded); err != nil {
			return nil, err
		}
		svg.WriteString("</metadata>")
	}
	svg.WriteString("</svg>")
	if svg.Len() > maxSVGBytes {
		return nil, errOutputLimit
	}
	return svg.Bytes(), nil
}
