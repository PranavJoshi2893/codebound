package viewer

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"strings"
	"testing"
)

func TestSeparateEdgeStrokesPreservesGraphAndMetadata(t *testing.T) {
	original := []byte(`<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="400" height="200">
<g class="node"><path fill="none" stroke="black" d="M0,0L5,5"/><text>A &amp; B</text></g>
<g class="edge"><title>A to B</title><a xlink:href="ignored"><path id="edge-path" fill="none" stroke="#2563eb" stroke-width="2" d="M5,5C10,5 20,10 30,10"/></a><polygon fill="blue" points="30,10 28,9 28,11"/></g>
<g class="node"><path fill="none" stroke="black" d="M30,30L35,35"/></g>
<g class="edge optional-class"><path fill="none" stroke="red" d="M0,10L30,0"/></g>
<metadata id="codebound-nodes">[{"id":0,"label":"A &amp; B","bounds":[1,2,30,40]}]</metadata></svg>`)
	decorated, err := separateEdgeStrokes(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(decorated), `class="edge-separation"`) != 2 || strings.Count(string(decorated), `id="edge-path"`) != 1 {
		t.Fatal("only edge paths should gain under-strokes, without duplicating SVG IDs")
	}
	for _, width := range []string{`stroke-width="5"`, `stroke-width="4"`} {
		if !strings.Contains(string(decorated), width) {
			t.Errorf("crossings need a narrow stroke wider than the actual line: %s", width)
		}
	}
	underStroke := regexp.MustCompile(`<path class="edge-separation"[^>]*></path>`)
	if cleaned := underStroke.ReplaceAll(decorated, nil); !bytes.Equal(cleaned, original) {
		t.Fatal("decoration must preserve every original byte, including transforms, source text, links, and node metadata")
	}
	var document struct{ XMLName xml.Name }
	if err := xml.Unmarshal(decorated, &document); err != nil {
		t.Fatalf("separated edges must remain valid SVG: %v", err)
	}
	nodes := decodeNodeMetadata(t, decorated)
	if len(nodes) != 1 || nodes[0].Label != "A & B" || nodes[0].Bounds != [4]int{1, 2, 30, 40} {
		t.Fatal("crossing decoration must not affect click targets")
	}
}

func TestSeparateEdgeStrokesLeavesNodeGeometryAlone(t *testing.T) {
	for _, svg := range []string{
		`<svg>graph</svg>`,
		`<svg><g class="node"><path fill="none" stroke="blue" d="M0,0L10,10"/></g></svg>`,
		`<svg><g class="edge"><path fill="blue" stroke="blue" d="M0,0L10,10"/></g></svg>`,
		`<svg><g class="edge"><path fill="none" stroke="none" d="M0,0L10,10"/></g></svg>`,
	} {
		result, err := separateEdgeStrokes([]byte(svg))
		if err != nil || string(result) != svg {
			t.Fatalf("non-edge geometry should be unchanged: %s, %v", result, err)
		}
	}
	if _, err := separateEdgeStrokes([]byte(`<svg><g class="edge"></svg>`)); err == nil {
		t.Fatal("malformed SVG must not be partially rewritten")
	}
}
