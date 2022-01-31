package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	// "github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type ModuleInfo struct {
	Source   string          `njson:"modules.0.source"`
	Versions []ModuleVersion `njson:"modules.0.versions"`
}

type ModuleVersion struct {
	Version   string                  `njson:"version"`
	Providers []ModuleVersionProvider `njson:"root.providers"`
}

type ModuleVersionProvider struct {
	Name      string `njson:"name"`
	Namespace string `njson:"namespace"`
	Source    string `njson:"source"`
	Version   string `njson:"version"`
}

type HCLFile struct {
	Module []HCLModule `hcl:"module,block"`
}

type HCLModule struct {
	Source  string
	Version string
}

func (module *HCLModule) Local() bool {
	return module.Source[0] == '.'
}

func (module *HCLModule) Remote() bool {
	return module.Source[0] != '.'
}

func GetAttributeValue(block *hclwrite.Block, attribute_name string) string {
	attr := block.Body().GetAttribute(attribute_name)
	if attr == nil {
		return ""
	}
	for _, t := range attr.BuildTokens(nil) {
		if t.Type == hclsyntax.TokenQuotedLit {
			return string(t.Bytes)
		}
	}
	return ""
}

func CreateModule(block *hclwrite.Block) HCLModule {
	src := GetAttributeValue(block, "source")
	ver := GetAttributeValue(block, "version")
	return HCLModule{
		Source:  src,
		Version: ver,
	}
}

// func DetermineModules(path string) (*HCLFile, error) {
// 	var hclfile HCLFile
// 	return &hclfile, hclsimple.DecodeFile(path, nil, &hclfile)
// }

func main() {
	// resp, err := http.Get("https://registry.terraform.io/v1/modules/terraform-aws-modules/vpc/aws/versions")
	// if err != nil {
	// 	panic(err)
	// }
	// var data ModuleInfo

	// body, readErr := ioutil.ReadAll(resp.Body)
	// if readErr != nil {
	// 	panic(readErr)
	// }

	// jsonErr := njson.Unmarshal(body, &data)
	// if jsonErr != nil {
	// 	panic(jsonErr)
	// }

	// sample_version, err := semver.NewVersion("2.0.0")
	// if err != nil {
	// 	panic(err)
	// }

	// var inFamilyVersions []*semver.Version
	// for _, ver := range data.Versions {
	// 	sv, err := semver.NewVersion(ver.Version)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	if sample_version.Major() == sv.Major() {
	// 		inFamilyVersions = append(inFamilyVersions, sv)
	// 	}
	// }
	// sort.Sort(semver.Collection(inFamilyVersions))
	// fmt.Printf("Version [%s] for module [%s] can be bumped to [%s]", sample_version.String(), data.Source, inFamilyVersions[len(inFamilyVersions)-1].String())
	// TODO: Cross reference provider requirements.
	// TODO: interpolate modules from terraform files.

	var path string
	path = "/tmp/terraform-aws-route53-recovery-controller/main.tf"
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var modules []HCLModule
	hf, _ := hclwrite.ParseConfig(data, path, hcl.InitialPos)
	for _, v := range hf.Body().Blocks() {
		if v.Type() == "module" {
			modules = append(modules, CreateModule(v))
		}
	}
	for _, b := range modules {
		fmt.Printf("Source: %s, Version: %s\n", b.Source, b.Version)
		if b.Local() {
			fmt.Println("- Local module")
		}
	}
}
