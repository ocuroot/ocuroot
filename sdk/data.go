package sdk

import (
	"fmt"
	"time"

	"github.com/ocuroot/ocuroot/refs"
	"go.starlark.net/starlark"
)

type WorkID string

type EnvironmentName string

type Environment struct {
	Name       EnvironmentName   `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

type Package struct {
	Functions map[string]Function `json:"functions"`

	Phases      []Phase                        `json:"phases"`
	Deployments map[EnvironmentName]Deployment `json:"deployments"`
}

type Phase struct {
	Name string `json:"name"`
	Work []Work `json:"work"`
}

type Work struct {
	Deployment *Deployment `json:"deploy,omitempty"`
	Call       *WorkCall   `json:"call,omitempty"`
}

type Deployment struct {
	Environment EnvironmentName `json:"environment"`

	Up   FunctionDef `json:"up"`
	Down FunctionDef `json:"down"`

	Inputs map[string]InputDescriptor `json:"inputs"`
}

type InputDescriptor struct {
	Ref     *refs.Ref `json:"ref,omitempty"`
	Default any       `json:"default,omitempty"`
	Value   any       `json:"value,omitempty"`
	Doc     *string   `json:"doc,omitempty"`
}

type Function struct {
	Function FunctionDef   `json:"function"`
	Graph    []HandoffEdge `json:"graph"`
}

type HandoffEdge struct {
	From          string `json:"from"`
	To            string `json:"to"`
	Annotation    string `json:"annotation"`
	Delay         string `json:"delay"`
	NeedsApproval bool   `json:"needs_approval,omitempty"`
}

type FunctionName string

func DefForFunction(fn *starlark.Function) FunctionDef {
	return FunctionDef{
		Name: FunctionName(fn.Name()),
		Pos:  fmt.Sprintf("%s:%d:%d", fn.Position().Filename(), fn.Position().Line, fn.Position().Col),
	}
}

type FunctionDef struct {
	Name FunctionName `json:"name"`
	Pos  string       `json:"pos"`
}

func (f FunctionDef) String() string {
	return fmt.Sprintf("%s/%s", f.Name, f.Pos)
}

type WorkResult struct {
	Next *WorkNext `json:"next,omitempty"`
	Done *WorkDone `json:"done,omitempty"`
	Err  error     `json:"error,omitempty"`
}

type WorkCall struct {
	Name   string                     `json:"name"`
	Inputs map[string]InputDescriptor `json:"inputs"`
	Fn     FunctionDef                `json:"fn,omitempty"`
}

type WorkNext struct {
	Fn     FunctionDef                `json:"fn,omitempty"`
	Inputs map[string]InputDescriptor `json:"inputs,omitempty"`
}

type WorkDone struct {
	Outputs map[string]any `json:"outputs"`
	Tags    []string       `json:"tags"`
}

type Log struct {
	Timestamp  time.Time         `json:"timestamp"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes"`
	Stream     int               `json:"stream"` // 1: stdout, 2: stderr
}
