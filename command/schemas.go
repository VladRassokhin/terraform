package command

import (
	"flag"
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
	return errorResult{resultBase{name, "unknown"}, "Not found"}
}
