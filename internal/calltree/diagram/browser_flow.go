package diagram

import (
	"fmt"
	"html"
	"strings"

	"github.com/PranavJoshi2893/codebound/internal/calltree/model"
)

// Each basic block is a small card INSIDE the existing function node. There is
// still one selectable function; call connectors attach to their source rows.
// Labeled exits, rather than vertical placement of block cards, define the path.
// Row links supply inert image-map geometry for tracing. They do not add
// selectable functions or executable SVG to the page.
func renderFlowLabel(header string, function model.Function, contexts, root bool, recursion map[string][]string, layout flowLayout, functionID int, registrations map[string][]string) string {
	var out strings.Builder
	out.WriteString(`<TABLE BORDER="0" CELLBORDER="0" CELLSPACING="0" CELLPADDING="0">`)
	fmt.Fprintf(&out, `<TR><TD COLSPAN="%d" PORT="entry" ALIGN="LEFT" BALIGN="LEFT" CELLPADDING="9">`, flowTableColumns(layout.columns)+2)
	for i, line := range strings.Split(header, "\n") {
		if i == 0 {
			fmt.Fprintf(&out, `<B>%s</B>`, html.EscapeString(line))
		} else {
			fmt.Fprintf(&out, `<BR ALIGN="LEFT"/><FONT POINT-SIZE="10" COLOR="#526075">%s</FONT>`, html.EscapeString(line))
		}
	}
	if contexts && root && function.Context != nil {
		for _, binding := range function.Context.Bindings {
			fmt.Fprintf(&out, `<BR ALIGN="LEFT"/><FONT POINT-SIZE="10" COLOR="#2563eb">input %s=%s</FONT>`,
				html.EscapeString(binding.Parameter), html.EscapeString(contextIDsLabel(binding.IDs, binding.Unresolved)))
		}
	}
	out.WriteString(`</TD></TR>`)
	operations := make(map[string][]model.ContextOperation)
	if contexts && function.Context != nil {
		for _, operation := range function.Context.Operations {
			operations[operation.Site] = append(operations[operation.Site], operation)
		}
	}
	for _, group := range layout.groups {
		renderFlowGroup(&out, layout, group, operations, recursion, functionID, registrations)
	}
	missing := 0
	for _, findings := range operations {
		missing += len(findings)
	}
	if missing > 0 {
		fmt.Fprintf(&out, `<TR><TD COLSPAN="%d" ALIGN="LEFT" CELLPADDING="6"><FONT POINT-SIZE="9" COLOR="#526075">%d context findings not positioned · see details</FONT></TD></TR>`, flowTableColumns(layout.columns)+2, missing)
	}
	unpositioned := 0
	for _, notes := range registrations {
		unpositioned += len(notes)
	}
	if unpositioned > 0 {
		fmt.Fprintf(&out, `<TR><TD COLSPAN="%d" ALIGN="LEFT" CELLPADDING="6"><FONT POINT-SIZE="9" COLOR="#92400e">%d route findings not positioned · see details</FONT></TD></TR>`, flowTableColumns(layout.columns)+2, unpositioned)
	}
	out.WriteString(`</TABLE>`)
	return out.String()
}

// Use a shared cell grid rather than nested tables: Graphviz stretches nested
// tables to the tallest sibling, distributing blank space between their rows.
// Direct text cells keep headers/steps top-aligned and short panels compact.
type flowSlot struct {
	scope *flowScopeLayout // nil is an empty column group
	panel flowPanel
	span  int
}

func flowTableColumns(span int) int { return 3*span - 1 } // reference + text + gutters

func flowRowStart(out *strings.Builder) {
	out.WriteString(`<TR><TD BORDER="0" WIDTH="6" CELLPADDING="0"></TD>`)
}

func flowRowEnd(out *strings.Builder) {
	out.WriteString(`<TD BORDER="0" WIDTH="6" CELLPADDING="0"></TD></TR>`)
}

func flowGutter(out *strings.Builder) {
	out.WriteString(`<TD BORDER="0" WIDTH="10" CELLPADDING="0"></TD>`)
}

func flowBlank(out *strings.Builder, span int) {
	fmt.Fprintf(out, `<TD COLSPAN="%d" BORDER="0" CELLPADDING="0"></TD>`, flowTableColumns(span))
}

func renderFlowGroup(out *strings.Builder, layout flowLayout, group []int, operations map[string][]model.ContextOperation, recursion map[string][]string, functionID int, registrations map[string][]string) {
	rows, width := 0, 0
	var titles []string
	var spans []int
	showTitles := len(group) > 1
	for _, id := range group {
		section := layout.scopes[id]
		span := section.columns
		if len(group) == 1 {
			span = layout.columns
		}
		spans = append(spans, span)
		width += span
		rows = max(rows, len(section.rows))
		title := fmt.Sprintf("ENTRY → B%d · follow exits", section.scope.Entry)
		if section.scope.ID != "body" {
			title = fmt.Sprintf("CLOSURE %s · only when invoked · entry B%d", section.scope.ID, section.scope.Entry)
		}
		if section.scope.Recover != nil {
			title += fmt.Sprintf("\nif panic recovered → B%d", *section.scope.Recover)
		}
		titles = append(titles, title)
		showTitles = showTitles || section.scope.ID != "body" || len(section.scope.Blocks) > 1 || section.columns > 1
	}
	if showTitles {
		flowRowStart(out)
		for i, title := range titles {
			if i > 0 {
				flowGutter(out)
			}
			fmt.Fprintf(out, `<TD COLSPAN="%d" BORDER="0" ALIGN="LEFT" VALIGN="TOP" BALIGN="LEFT" CELLPADDING="6"><FONT POINT-SIZE="9" COLOR="#526075">%s</FONT></TD>`, flowTableColumns(spans[i]), strings.ReplaceAll(html.EscapeString(title), "\n", `<BR ALIGN="LEFT"/>`))
		}
		if width < layout.columns {
			flowGutter(out)
			flowBlank(out, layout.columns-width)
		}
		flowRowEnd(out)
	}
	for row := 0; row < rows; row++ {
		var slots []flowSlot
		for i, id := range group {
			section := &layout.scopes[id]
			if row >= len(section.rows) {
				slots = append(slots, flowSlot{span: spans[i]})
				continue
			}
			panels := section.rows[row]
			for _, panel := range panels {
				span := 1
				if len(panels) == 1 {
					span = spans[i]
				}
				slots = append(slots, flowSlot{scope: section, panel: panel, span: span})
			}
			if len(panels) > 1 && len(panels) < spans[i] {
				slots = append(slots, flowSlot{span: spans[i] - len(panels)})
			}
		}
		if width < layout.columns {
			slots = append(slots, flowSlot{span: layout.columns - width})
		}
		renderFlowBand(out, slots, operations, recursion, functionID, registrations)
		fmt.Fprintf(out, `<TR><TD COLSPAN="%d" BORDER="0" HEIGHT="8" CELLPADDING="0"></TD></TR>`, flowTableColumns(layout.columns)+2)
	}
}

func renderFlowBand(out *strings.Builder, slots []flowSlot, operations map[string][]model.ContextOperation, recursion map[string][]string, functionID int, registrations map[string][]string) {
	flowRowStart(out)
	steps := 0
	for i, slot := range slots {
		if i > 0 {
			flowGutter(out)
		}
		if slot.scope == nil {
			flowBlank(out, slot.span)
			continue
		}
		steps = max(steps, slot.panel.last-slot.panel.first)
		renderPanelHeader(out, slot.scope.scope.Blocks[slot.panel.block], slot.panel, slot.span)
	}
	flowRowEnd(out)
	for row := 0; row <= steps; row++ {
		flowRowStart(out)
		for i, slot := range slots {
			if i > 0 {
				flowGutter(out)
			}
			count := slot.panel.last - slot.panel.first
			if slot.scope == nil || row > count {
				flowBlank(out, slot.span)
				continue
			}
			block := slot.scope.scope.Blocks[slot.panel.block]
			if row == count {
				renderPanelExit(out, block, slot.panel, slot.span)
			} else {
				renderPanelStep(out, block.Steps[slot.panel.first+row], slot.span, operations, recursion, functionID, registrations)
			}
		}
		flowRowEnd(out)
	}
}

func renderPanelHeader(out *strings.Builder, block model.FlowBlock, panel flowPanel, span int) {
	title := fmt.Sprintf("B%d · %s", block.ID, block.Label)
	if block.Label == "" {
		title = fmt.Sprintf("B%d", block.ID)
	}
	color := "#f1f5f9"
	if block.Exit == "branch" && panel.last == len(block.Steps) {
		color = "#fef3c7"
	}
	fmt.Fprintf(out, `<TD COLSPAN="%d" BORDER="1" SIDES="TLR" COLOR="#dbe2ea" CELLPADDING="5" ALIGN="LEFT" VALIGN="TOP" BALIGN="LEFT" BGCOLOR="%s"><FONT POINT-SIZE="10"><B>%s</B></FONT>`, flowTableColumns(span), color, html.EscapeString(title))
	if panel.first > 0 || panel.last < len(block.Steps) {
		fmt.Fprintf(out, `<BR ALIGN="LEFT"/><FONT POINT-SIZE="9" COLOR="#526075">steps %d–%d ↓</FONT>`, panel.first+1, panel.last)
	}
	out.WriteString(`</TD>`)
}

func renderPanelStep(out *strings.Builder, step model.FlowStep, span int, operations map[string][]model.ContextOperation, recursion map[string][]string, functionID int, registrations map[string][]string) {
	label, background := step.Label, "white"
	switch step.Kind {
	case "go":
		label, background = "go · concurrent · "+label, "#ecfeff"
	case "defer":
		label, background = "register defer · "+label, "#fff7ed"
	case "run_defers":
		background = "#fff7ed"
	}
	if step.Closure != "" {
		label += " → closure " + step.Closure
	}
	port, leftPort := "", ""
	if step.ID != "" {
		port = ` PORT="` + html.EscapeString(step.ID) + `"`
		leftPort = ` PORT="` + html.EscapeString(step.ID+"_w") + `"`
	}
	if functionID >= 0 && step.ID != "" {
		port += fmt.Sprintf(` HREF="site:%d:%s" TOOLTIP="%s"`, functionID, html.EscapeString(step.ID), html.EscapeString(step.Label))
	}
	fmt.Fprintf(out, `<TD%s BORDER="1" SIDES="L" COLOR="#dbe2ea" CELLPADDING="5" ALIGN="LEFT" VALIGN="TOP" BGCOLOR="%s"><FONT POINT-SIZE="9" COLOR="#64748b">%s</FONT></TD><TD%s COLSPAN="%d" BORDER="1" SIDES="R" COLOR="#dbe2ea" CELLPADDING="5" ALIGN="LEFT" VALIGN="TOP" BALIGN="LEFT" BGCOLOR="%s"><FONT POINT-SIZE="11">%s</FONT>`, leftPort, background, html.EscapeString(step.Reference), port, flowTableColumns(span)-1, background, html.EscapeString(label))
	for i, note := range recursion[step.ID] {
		color := "#2563eb"
		if i == 0 {
			color = "#7c3aed"
		}
		fmt.Fprintf(out, `<BR ALIGN="LEFT"/><FONT POINT-SIZE="10" COLOR="%s">%s</FONT>`, color, html.EscapeString(note))
	}
	for _, note := range registrations[step.ID] {
		fmt.Fprintf(out, `<BR ALIGN="LEFT"/><FONT POINT-SIZE="10" COLOR="#92400e">%s</FONT>`, html.EscapeString(note))
	}
	delete(registrations, step.ID)
	var findings []model.ContextOperation
	if step.ID != "" {
		findings = operations[step.ID]
		delete(operations, step.ID)
	}
	seen := make(map[string]bool)
	for _, operation := range findings {
		note := flowContextLabel(operation)
		if !seen[note] {
			seen[note] = true
			fmt.Fprintf(out, `<BR ALIGN="LEFT"/><FONT POINT-SIZE="10" COLOR="#2563eb">%s</FONT>`, html.EscapeString(note))
		}
	}
	out.WriteString(`</TD>`)
}

func renderPanelExit(out *strings.Builder, block model.FlowBlock, panel flowPanel, span int) {
	footer := flowExitLabel(block)
	if panel.last < len(block.Steps) {
		reference := block.Steps[panel.last].Reference
		if reference == "" {
			reference = fmt.Sprintf("B%d.%d", block.ID, panel.last+1)
		}
		footer = "continue → " + reference
	}
	fmt.Fprintf(out, `<TD COLSPAN="%d" BORDER="1" SIDES="BLR" COLOR="#dbe2ea" BGCOLOR="white" CELLPADDING="5" ALIGN="LEFT" VALIGN="TOP" BALIGN="LEFT"><FONT POINT-SIZE="10" COLOR="#475569">%s</FONT></TD>`, flowTableColumns(span), strings.ReplaceAll(html.EscapeString(footer), "\n", `<BR ALIGN="LEFT"/>`))
}

func flowExitLabel(block model.FlowBlock) string {
	switch block.Exit {
	case "return":
		return "return"
	case "panic":
		return "panic · unwind deferred stack"
	}
	var targets []string
	for _, next := range block.Next {
		label := next.Label
		if next.BackEdge {
			if label != "" {
				label += " · "
			}
			label += "loop"
		}
		if label != "" {
			label += " "
		}
		targets = append(targets, fmt.Sprintf("%s→ B%d", label, next.To))
	}
	if block.Exit == "branch" {
		return "if " + block.Condition + "\n" + strings.Join(targets, " | ")
	}
	if len(targets) > 0 {
		return strings.Join(targets, " | ")
	}
	return "stop"
}

func flowContextLabel(operation model.ContextOperation) string {
	ids := contextIDsLabel(operation.IDs, operation.Unresolved)
	var label string
	switch operation.Kind {
	case "create":
		label = "new " + ids
	case "derive", "detach":
		label = ids + " ← " + contextIDsLabel(operation.ParentIDs, operation.ParentUnresolved)
		if operation.Kind == "detach" {
			label += " · cancellation detached"
		}
	case "cancel":
		label = "cancel/stop " + ids
	case "forward":
		label = "passes " + ids
	case "observe":
		label = "observes Done: " + ids
	case "read":
		label = "reads " + shortContextTarget(operation.Target) + ": " + ids
	default:
		label = operation.Kind + " " + ids
	}
	if operation.Execution == "deferred" {
		label = "on exit: " + label
	} else if operation.Execution == "goroutine" {
		label = "concurrent: " + label
	}
	return label
}
