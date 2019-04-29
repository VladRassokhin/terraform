package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hil/ast"
	backendlocal "github.com/hashicorp/terraform/backend/local"
	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/plugin/discovery"
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

type providerResourceSchema struct {
	resultBase
	*terraform.ResourceProviderSchema
}

type resourceResourceSchema struct {
	resultBase
	Provider string                           `json:"provider"`
	Schema   terraform.SchemaInfoWithTimeouts `json:"schema"`
}

type provisionerResourceSchemaInfo struct {
	resultBase
	*terraform.ResourceProvisionerSchema
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

type allSchema struct {
	resultBase
	Names []string `json:"names"`
}

func (c *SchemasCommand) Run(args []string) int {
	var indent bool
	var inJson bool
	var inXml bool
	var expectedType string

	args, err := c.Meta.process(args, false)
	if err != nil {
		return 1
	}

	cmdFlags := flag.NewFlagSet("schemas", flag.ContinueOnError)
	cmdFlags.BoolVar(&indent, "indent", false, "Indent output")
	cmdFlags.BoolVar(&inJson, "json", false, "In JSON format")
	cmdFlags.StringVar(&expectedType, "type", "any", "Type of object: provisioner, provider, resource, "+
		"data-source, function. Should be specified if there's two objects with same name")
	expectedType = strings.ToLower(expectedType)
	// Temporarily disabled due to not-implemented xml serializer for SchemaInfo (which is map[string]interface{})
	//cmdFlags.BoolVar(&inXml, "xml", false, "In XML format")
	cmdFlags.Usage = func() { c.Ui.Error(c.Help()) }
	if err := cmdFlags.Parse(args); err != nil {
		c.Ui.Error("Cannot parser command line arguments" + err.Error())
		cmdFlags.Usage()
		return 1
	}

	if inXml && inJson {
		c.Ui.Error("Cannot produce output in both xml in json formats at the same time. Either use -josn or -xml flags")
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
	b, err := c.Backend(&BackendOpts{
		ForceLocal: true,
	})
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to load backend: %s", err))
		return 1
	}
	localBackend, ok := b.(*backendlocal.Local)
	if !ok {
		c.Ui.Error(fmt.Sprintf("Failed to load backend: %s", err))
		return 1
	}
	s = getOrErrorResult(localBackend.ContextOpts, args[0], expectedType)

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

  -type	<type>            If specified, would search for specific type, e.g. provider, resource, provisioner or function.
`
	return strings.TrimSpace(helpText)
}

func (c *SchemasCommand) Synopsis() string {
	return "Shows schemas of Terraform providers/resources"
}

func getOrErrorResult(context *terraform.ContextOpts, name string, expectedType string) interface{} {
	var s interface{}
	var e error
	switch expectedType {
	case "function":
		s = getFunctionSchema(name)
	case "provider":
		s, e = getProviderSchema(context.ProviderResolver, name)
	case "resource":
		s, e = getResourceSchema(context.ProviderResolver, name)
	case "data-source":
		s, e = getDataSourceSchema(context.ProviderResolver, name)
	case "provisioner":
		s, e = getProvisionerSchema(context.Provisioners, name)
	case "any":
		s = getAnythingOrErrorResult(context, name)
	default:
		return errorResult{resultBase{name, "unknown"}, "Unexpected type " + expectedType}
	}
	if e != nil {
		return errorResult{resultBase{name, expectedType}, e.Error()}
	} else if s != nil {
		return s
	}
	return errorResult{resultBase{name, expectedType}, "Not found"}
}

func getAnythingOrErrorResult(context *terraform.ContextOpts, name string) interface{} {
	if name == "functions" {
		return functionsSchema{resultBase{"functions", "functions"}, getInterpolationFunctions()}
	}
	var names = getAllNames(context, name)
	if names != nil {
		return allSchema{resultBase{name, "names"}, names}
	}
	var a interface{}
	var e error
	a, e = getProviderSchema(context.ProviderResolver, name)
	if e != nil {
		return errorResult{resultBase{name, "provider"}, e.Error()}
	} else if a != nil {
		return a
	}
	a, e = getResourceSchema(context.ProviderResolver, name)
	if e != nil {
		return errorResult{resultBase{name, "resource"}, e.Error()}
	} else if a != nil {
		return a
	}
	a, e = getDataSourceSchema(context.ProviderResolver, name)
	if e != nil {
		return errorResult{resultBase{name, "data-source"}, e.Error()}
	} else if a != nil {
		return a
	}
	a, e = getProvisionerSchema(context.Provisioners, name)
	if e != nil {
		return errorResult{resultBase{name, "provisioner"}, e.Error()}
	} else if a != nil {
		return a
	}
	return errorResult{resultBase{name, "unknown"}, "Not found"}
}

func getInterpolationFunctions() map[string]FunctionInfo {
	vars := make(map[string]ast.Variable)
	cfg := config.LangEvalConfig(vars)
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
	cfg := config.LangEvalConfig(vars)
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

func getFunctionSchema(name string) interface{} {
	function, found := getInterpolationFunction(name)
	if !found {
		return nil
	}
	return functionSchema{resultBase{name, "function"}, *function}
}

func getProvider(resolver terraform.ResourceProviderResolver, name string) (map[string]terraform.ResourceProviderFactory, error) {
	req := make(discovery.PluginRequirements)
	req[name] = &discovery.PluginConstraints{
		Versions: discovery.Constraints{},
	}
	providers, err := resolver.ResolveProviders(req)
	if err != nil {
		return nil, &multierror.Error{
			Errors: err,
		}
	}
	return providers, nil
}

func getProviderName(name string) string {
	i := strings.Index(name, "_")
	if i == -1 {
		return name
	}
	return name[0:i]
}

func getProviderSchema(resolver terraform.ResourceProviderResolver, name string) (interface{}, error) {
	providers, err := getProvider(resolver, name)
	if err != nil {
		return nil, err
	}
	for k, v := range providers {
		if name != k {
			continue
		}
		if provider, err := v(); err == nil {
			export, err := provider.Export()
			if err != nil {
				return nil, fmt.Errorf("Cannot get schema for provider '%s': %s\n", k, err)
			}
			extended := providerResourceSchema{resultBase{k, "provider"}, export}
			return extended, nil
		}
	}
	return nil, nil
}

func getResourceSchema(resolver terraform.ResourceProviderResolver, name string) (interface{}, error) {
	providers, err := getProvider(resolver, getProviderName(name))
	if err != nil {
		return nil, err
	}
	for k, v := range providers {
		if provider, err := v(); err == nil {
			resources := provider.Resources()
			for _, n := range resources {
				if n.Name != name {
					continue
				}
				export, err := provider.Export()
				if err != nil {
					return nil, fmt.Errorf("Cannot get schema for resource '%s': %s\n", k, err)
				}
				extended := resourceResourceSchema{resultBase{k, "resource"}, k, export.Resources[n.Name]}
				return extended, nil
			}
		}
	}
	return nil, nil
}

func getDataSourceSchema(resolver terraform.ResourceProviderResolver, name string) (interface{}, error) {
	providers, err := getProvider(resolver, getProviderName(name))
	if err != nil {
		return nil, err
	}
	for k, v := range providers {
		if provider, err := v(); err == nil {
			data_sources := provider.DataSources()
			for _, n := range data_sources {
				if n.Name != name {
					continue
				}
				export, err := provider.Export()
				if err != nil {
					return nil, fmt.Errorf("Cannot get schema for resource '%s': %s\n", k, err)
				}
				extended := resourceResourceSchema{resultBase{k, "data-source"}, k, export.DataSources[n.Name]}
				return extended, nil
			}
		}
	}
	return nil, nil
}

func getProvisionerSchema(providers map[string]terraform.ResourceProvisionerFactory, name string) (interface{}, error) {
	for k, v := range providers {
		if name != k {
			continue
		}
		if provisioner, err := v(); err == nil {
			export, err := provisioner.Export()
			if err != nil {
				return nil, fmt.Errorf("Cannot get schema for provisioner '%s': %s\n", k, err)
			}
			extended := provisionerResourceSchemaInfo{resultBase{k, "provisioner"}, export}
			return extended, nil
		}
	}
	return nil, nil
}

func getAllNames(context *terraform.ContextOpts, name string) []string {
	//if name == "providers" {
	//	return getAllProviders(context)
	//}
	if name == "provisioners" {
		return getAllProvisioners(context)
	}
	//if name == "resources" {
	//	return getAllResources(context)
	//}
	//if name == "data-sources" {
	//	return getAllDataSources(context)
	//}
	return nil
}

//func getAllProviders(context *terraform.ContextOpts) []string {
//	providers := context.Providers
//	result := make([]string, 0, len(providers))
//	for name := range providers {
//		result = append(result, name)
//	}
//	return result
//}

func getAllProvisioners(context *terraform.ContextOpts) []string {
	provisioners := context.Provisioners
	result := make([]string, 0, len(provisioners))
	for name := range provisioners {
		result = append(result, name)
	}
	return result
}

//func getAllResources(context *terraform.ContextOpts) []string {
//	providers := context.Providers
//	result := make([]string, 0)
//	for _, v := range providers {
//		if provider, err := v(); err == nil {
//			resources := provider.Resources()
//			for _, r := range resources {
//				result = append(result, r.Name)
//			}
//		}
//	}
//	return result
//}
//func getAllDataSources(context *terraform.ContextOpts) []string {
//	providers := context.Providers
//	result := make([]string, 0)
//	for _, v := range providers {
//		if provider, err := v(); err == nil {
//			sources := provider.DataSources()
//			for _, ds := range sources {
//				result = append(result, ds.Name)
//			}
//		}
//	}
//	return result
//}
