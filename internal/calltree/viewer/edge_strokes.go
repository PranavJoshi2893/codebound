package viewer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Crossing routes must not look electrically joined. Draw a narrow white
// under-stroke behind each edge, so later routes visibly pass over earlier ones.
// This adds no filters, scripts, DOM SVG, or second layout. All original SVG
// bytes (including node coordinates/metadata) are retained verbatim.
func separateEdgeStrokes(svg []byte) ([]byte, error) {
	if len(svg) > maxSVGBytes {
		return nil, errOutputLimit
	}
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	output := limitedBuffer{limit: maxSVGBytes}
	encoder := xml.NewEncoder(&output)
	depth, edgeDepth, copied := 0, 0, 0
	for {
		start := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode graph edge geometry: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			depth++
			if token.Name.Local != "g" && (edgeDepth == 0 || token.Name.Local != "path") {
				continue
			}
			attributes := make(map[string]string, len(token.Attr))
			for _, attribute := range token.Attr {
				if attribute.Name.Space == "" {
					attributes[attribute.Name.Local] = attribute.Value
				}
			}
			if token.Name.Local == "g" && edgeDepth == 0 {
				for _, class := range strings.Fields(attributes["class"]) {
					if class == "edge" {
						edgeDepth = depth
					}
				}
			}
			if edgeDepth == 0 || token.Name.Local != "path" || attributes["d"] == "" ||
				attributes["stroke"] == "" || attributes["stroke"] == "none" || attributes["fill"] != "none" {
				continue
			}
			if _, err := output.Write(svg[copied:start]); err != nil {
				return nil, err
			}
			width, err := strconv.ParseFloat(attributes["stroke-width"], 64)
			if err != nil || math.IsNaN(width) || math.IsInf(width, 0) || width <= 0 {
				width = 1
			}
			path := xml.StartElement{Name: xml.Name{Local: "path"}, Attr: []xml.Attr{
				{Name: xml.Name{Local: "class"}, Value: "edge-separation"},
				{Name: xml.Name{Local: "d"}, Value: attributes["d"]},
				{Name: xml.Name{Local: "fill"}, Value: "none"},
				{Name: xml.Name{Local: "stroke"}, Value: "white"},
				{Name: xml.Name{Local: "stroke-width"}, Value: strconv.FormatFloat(width+3, 'f', -1, 64)},
				{Name: xml.Name{Local: "stroke-linecap"}, Value: "round"},
				{Name: xml.Name{Local: "stroke-linejoin"}, Value: "round"},
			}}
			if err := encoder.EncodeToken(path); err != nil {
				return nil, err
			}
			if err := encoder.EncodeToken(path.End()); err != nil {
				return nil, err
			}
			if err := encoder.Flush(); err != nil {
				return nil, err
			}
			copied = start
		case xml.EndElement:
			if depth == edgeDepth {
				edgeDepth = 0
			}
			depth--
		}
	}
	if output.buffer.Len() == 0 {
		return svg, nil
	}
	if _, err := output.Write(svg[copied:]); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
