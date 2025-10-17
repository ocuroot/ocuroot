package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"go.starlark.net/starlark"
)

// RenderValue pretty-prints a Starlark value for the REPL
func RenderValue(in starlark.Value) string {
	return renderValueWithIndent(in, 0)
}

func renderValueWithIndent(in starlark.Value, indent int) string {
	if in == nil || in == starlark.None {
		return "None"
	}

	// Handle primitives (int, float, string, bool)
	switch v := in.(type) {
	case starlark.Bool, starlark.Int, starlark.Float, starlark.String, starlark.Bytes:
		return in.String()

	case *starlark.List:
		return renderList(v, indent)

	case starlark.Tuple:
		return renderTuple(v, indent)

	case *starlark.Dict:
		return renderDict(v, indent)

	case *starlark.Set:
		return renderSet(v, indent)

	case *starlark.Function:
		return renderFunction(v, indent)

	case *starlark.Builtin:
		return renderBuiltin(v, indent)

	default:
		// Check if it's a struct-like object with attributes
		if hasAttrs, ok := in.(starlark.HasAttrs); ok {
			return renderStruct(hasAttrs, indent)
		}
		// Fallback for any other types
		return in.String()
	}
}

// renderList renders a list with tree-like structure
func renderList(list *starlark.List, indent int) string {
	if list.Len() == 0 {
		return "[]"
	}

	var sb strings.Builder
	sb.WriteString("[\n")

	iter := list.Iterate()
	defer iter.Done()
	var val starlark.Value
	i := 0
	for iter.Next(&val) {
		isLast := i == list.Len()-1
		prefix := getTreePrefix(indent, isLast)
		
		sb.WriteString(prefix)
		sb.WriteString(renderValueWithIndent(val, indent+1))
		sb.WriteString("\n")
		i++
	}

	sb.WriteString(strings.Repeat("  ", indent))
	sb.WriteString("]")
	return sb.String()
}

// renderTuple renders a tuple with tree-like structure
func renderTuple(tuple starlark.Tuple, indent int) string {
	if tuple.Len() == 0 {
		return "()"
	}

	var sb strings.Builder
	sb.WriteString("(\n")

	for i := 0; i < tuple.Len(); i++ {
		isLast := i == tuple.Len()-1
		prefix := getTreePrefix(indent, isLast)
		
		sb.WriteString(prefix)
		sb.WriteString(renderValueWithIndent(tuple.Index(i), indent+1))
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("  ", indent))
	sb.WriteString(")")
	return sb.String()
}

// renderDict renders a dictionary with tree-like structure
func renderDict(dict *starlark.Dict, indent int) string {
	if dict.Len() == 0 {
		return "{}"
	}

	var sb strings.Builder
	sb.WriteString("{\n")

	items := dict.Items()
	for i, item := range items {
		isLast := i == len(items)-1
		prefix := getTreePrefix(indent, isLast)
		
		key := item.Index(0)
		val := item.Index(1)
		
		sb.WriteString(prefix)
		sb.WriteString(renderValueWithIndent(key, indent+1))
		sb.WriteString(": ")
		sb.WriteString(renderValueWithIndent(val, indent+1))
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("  ", indent))
	sb.WriteString("}")
	return sb.String()
}

// renderSet renders a set with tree-like structure
func renderSet(set *starlark.Set, indent int) string {
	if set.Len() == 0 {
		return "set([])"
	}

	var sb strings.Builder
	sb.WriteString("set([\n")

	iter := set.Iterate()
	defer iter.Done()
	var val starlark.Value
	i := 0
	length := set.Len()
	for iter.Next(&val) {
		isLast := i == length-1
		prefix := getTreePrefix(indent, isLast)
		
		sb.WriteString(prefix)
		sb.WriteString(renderValueWithIndent(val, indent+1))
		sb.WriteString("\n")
		i++
	}

	sb.WriteString(strings.Repeat("  ", indent))
	sb.WriteString("])")
	return sb.String()
}

// renderFunction renders a Starlark function with its signature
func renderFunction(fn *starlark.Function, indent int) string {
	var sb strings.Builder
	
	// Function name
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	sb.WriteString(style.Render(fn.Name()))
	sb.WriteString("(")

	// Parameters
	params := make([]string, 0, fn.NumParams())
	for i := 0; i < fn.NumParams(); i++ {
		paramName, _ := fn.Param(i)
		
		// Check if this is *args or **kwargs
		if i == fn.NumParams()-2 && fn.HasVarargs() && fn.HasKwargs() {
			params = append(params, "*"+paramName)
		} else if i == fn.NumParams()-1 && fn.HasKwargs() {
			params = append(params, "**"+paramName)
		} else if i == fn.NumParams()-1 && fn.HasVarargs() {
			params = append(params, "*"+paramName)
		} else {
			// Check for default value
			if dflt := fn.ParamDefault(i); dflt != nil {
				params = append(params, fmt.Sprintf("%s=%v", paramName, dflt))
			} else {
				params = append(params, paramName)
			}
		}
	}

	sb.WriteString(strings.Join(params, ", "))
	sb.WriteString(")")

	// Add doc string only if at top level (indent 0)
	if indent == 0 && fn.Doc() != "" {
		sb.WriteString("\n  ")
		docStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
		sb.WriteString(docStyle.Render(fmt.Sprintf(`"""%s"""`, fn.Doc())))
	}

	return sb.String()
}

// renderBuiltin renders a builtin function
func renderBuiltin(b *starlark.Builtin, indent int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
	name := b.Name()
	
	if recv := b.Receiver(); recv != nil {
		return fmt.Sprintf("%s.%s(...)", recv.Type(), style.Render(name))
	}
	
	return style.Render(name) + "(...)"
}

// renderStruct renders a struct-like object with attributes as a tree
func renderStruct(obj starlark.HasAttrs, indent int) string {
	attrNames := obj.AttrNames()
	if len(attrNames) == 0 {
		// No attributes, just show the type
		return obj.(starlark.Value).Type()
	}

	var sb strings.Builder
	
	// Show type at the top
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	sb.WriteString(typeStyle.Render(obj.(starlark.Value).Type()))
	sb.WriteString(" {\n")

	// Render each attribute
	for i, name := range attrNames {
		isLast := i == len(attrNames)-1
		prefix := getTreePrefix(indent, isLast)
		
		sb.WriteString(prefix)
		sb.WriteString(name)
		sb.WriteString(": ")
		
		// Try to get the attribute value
		if val, err := obj.Attr(name); err == nil && val != nil {
			sb.WriteString(renderValueWithIndent(val, indent+1))
		} else {
			sb.WriteString("<unavailable>")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat("  ", indent))
	sb.WriteString("}")
	return sb.String()
}

// getTreePrefix returns the tree-drawing prefix for the given indent level
func getTreePrefix(indent int, isLast bool) string {
	if indent == 0 {
		if isLast {
			return "  └─ "
		}
		return "  ├─ "
	}

	prefix := strings.Repeat("  ", indent)
	if isLast {
		return prefix + "└─ "
	}
	return prefix + "├─ "
}
