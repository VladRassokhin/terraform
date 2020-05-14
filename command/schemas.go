package command

import (
	"flag"
	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/hashicorp/terraform/backend/init"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"strings"
)

// SchemasCommand is a Command implementation that reads and outputs the
// schemas of all installed Terraform providers and resource types.
type SchemasCommand struct {
	Meta
}

type resultBase struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type errorResult struct {
	resultBase
	Error string `json:"error"`
}

type FunctionInfo struct {
	Name         string
	ArgTypes     []string
	ReturnType   string
	Variadic     bool   `json:",omitempty"`
	VariadicType string `json:",omitempty"`
}

type functionSchema struct {
	resultBase
	FunctionInfo `json:"schema"`
}

type functionsSchema struct {
	resultBase
	Functions map[string]FunctionInfo `json:"schema"`
}

type backendSchema struct {
	resultBase
	Schema terraform.SchemaInfo `json:"schema"`
}

func (c *SchemasCommand) Run(args []string) int {
	var indent bool
	var inJson bool
	var inXml bool

	args = c.Meta.process(args)

	cmdFlags := flag.NewFlagSet("schemas", flag.ContinueOnError)
	cmdFlags.BoolVar(&indent, "indent", false, "Indent output")
	cmdFlags.BoolVar(&inJson, "json", false, "In JSON format")
	// Temporarily disabled due to not-implemented xml serializer for SchemaInfo (which is map[string]interface{})
	//cmdFlags.BoolVar(&inXml, "xml", false, "In XML format")
	cmdFlags.Usage = func() { c.Ui.Error(c.Help()) }
	if err := cmdFlags.Parse(args); err != nil {
		c.Ui.Error("Cannot parser command line arguments" + err.Error())
		cmdFlags.Usage()
		return 1
	}

	if inXml && inJson {
		c.Ui.Error("Cannot produce output in both xml in json formats at the same time. Either use -json or -xml flags")
		return 1
	}

	if inXml || inJson {
		c.color = false
	}
	var format string
	if inJson {
		format = "json"
	} else if inXml {
		format = "xml"
	} else {
		format = "plain"
	}

	args = cmdFlags.Args()
	if len(args) != 1 {
		c.Ui.Error("The schemas command expects one argument with the type of provider/resource.")
		cmdFlags.Usage()
		return 1
	}

	var s interface{}
	s = getAnythingOrErrorResult(args[0])

	c.Ui.Output(FormatSchema(&FormatSchemaOpts{
		Name:      args[0],
		Schema:    &s,
		Colorize:  c.color,
		Colorizer: c.Colorize(),
		Format:    format,
		Indent:    indent,
	}))

	// Return non-zero xit code in case of error (error result)
	switch s.(type) {
	case errorResult:
		return 1
	default:
		return 0
	}
}

func (c *SchemasCommand) Help() string {
	helpText := `
Usage: terraform schemas [options] name

  Reads and outputs the schema of specified ('name') Terraform provider,
  provisioner or resource in machine- or human-readable form.

Options:

  -indent		      If specified, output would be indented.

  -json		          If specified, output would be in JSON format. Implies '--no-color'.
`
	return strings.TrimSpace(helpText)
}

func (c *SchemasCommand) Synopsis() string {
	return "Shows schemas of Terraform providers/resources"
}

func getAnythingOrErrorResult(name string) interface{} {
	if name == "functions" {
		return functionsSchema{resultBase{"functions", "functions"}, getInterpolationFunctions()}
	}
	s := getFunctionSchema(name)
	if s != nil {
		return s
	}
	s = getBackendSchema(name)
	if s != nil {
		return s
	}
	return errorResult{resultBase{name, "unknown"}, "Not found"}
}

func getInterpolationFunctions() map[string]FunctionInfo {
	vars := make(map[string]ast.Variable)
	cfg := getLangEvalConfig(vars)
	result := make(map[string]FunctionInfo)
	for name, fun := range cfg.GlobalScope.FuncMap {
		args := make([]string, len(fun.ArgTypes))
		for i, at := range fun.ArgTypes {
			args[i] = at.String()
		}
		vt := ""
		if fun.Variadic {
			vt = fun.VariadicType.String()
		}
		result[name] = FunctionInfo{name, args, fun.ReturnType.String(), fun.Variadic, vt}
	}

	return result
}

func getInterpolationFunction(name string) (*FunctionInfo, bool) {
	vars := make(map[string]ast.Variable)
	cfg := getLangEvalConfig(vars)
	fm := cfg.GlobalScope.FuncMap
	if fun, ok := fm[name]; ok {
		args := make([]string, len(fun.ArgTypes))
		for i, at := range fun.ArgTypes {
			args[i] = at.String()
		}
		vt := ""
		if fun.Variadic {
			vt = fun.VariadicType.String()
		}
		return &FunctionInfo{name, args, fun.ReturnType.String(), fun.Variadic, vt}, true
	}
	return nil, false
}

func getLangEvalConfig(vars map[string]ast.Variable) hil.EvalConfig {
	return hil.EvalConfig{
		GlobalScope: &ast.BasicScope{
			VarMap: vars,
		},
	}
}

func getFunctionSchema(name string) interface{} {
	function, found := getInterpolationFunction(name)
	if !found {
		return nil
	}
	return functionSchema{resultBase{name, "function"}, *function}
}

func getBackend(name string) (*terraform.SchemaInfo, bool) {
	fn := init.Backend(name)
	if fn == nil {
		return nil, false
	}
	backend := fn()

	if b, isSchemaBackend := (backend.(interface{})).(*schema.Backend); isSchemaBackend {
		im := schema.InternalMap(b.Schema)
		info := im.Export()
		return &info, true
	} else {
		s := backend.ConfigSchema()
		info := schema.ExportBlock(s)
		return &info, true
	}
}

func getBackendSchema(name string) interface{} {
	backend, found := getBackend(name)
	if !found {
		return nil
	}
	return backendSchema{resultBase{name, "backend"}, *backend}
}
