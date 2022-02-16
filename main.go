package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	// "github.com/hashicorp/hcl/v2/hclsimple"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/zclconf/go-cty/cty"

	"github.com/Masterminds/semver"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/m7shapan/njson"
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
	Block   *hclwrite.Block
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

func SetVersionAttribute(block *hclwrite.Block, attribute_value *semver.Version) bool {
	body := block.Body()
	body.SetAttributeValue("version", cty.StringVal(attribute_value.String()))
	return true
}

func CreateModule(block *hclwrite.Block) HCLModule {
	src := GetAttributeValue(block, "source")
	ver := GetAttributeValue(block, "version")
	return HCLModule{
		Source:  src,
		Version: ver,
		Block:   block,
	}
}


func GetModuleVersionsFromRegistry(registry_path string) []*semver.Version {
	resp, err := http.Get("https://registry.terraform.io/v1/modules/" + registry_path + "/versions")
	if err != nil {
		panic(err)
	}
	var data ModuleInfo

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		panic(readErr)
	}

	jsonErr := njson.Unmarshal(body, &data)
	if jsonErr != nil {
		panic(jsonErr)
	}

	var ModuleSemVersions []*semver.Version
	for _, ver := range data.Versions {
		sv, err := semver.NewVersion(ver.Version)
		if err != nil {
			panic(err)
		}
		ModuleSemVersions = append(ModuleSemVersions, sv)
	}
	return ModuleSemVersions
	// TODO: Cross reference provider requirements.
	// TODO: interpolate modules from terraform files.
}

func main() {

	var path string
	path = "/tmp/terraform-aws-rds-aurora/deploy/main.tf"
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
		var inFamilyVersions []*semver.Version
		if b.Local() {
			continue
		}
		currentVersion, err := semver.NewVersion(b.Version)
		if err != nil {
			panic(err)
		}
		for _, v := range GetModuleVersionsFromRegistry(b.Source) {
			if currentVersion.Major() == v.Major() {
				inFamilyVersions = append(inFamilyVersions, v)
			}
		}
		sort.Sort(semver.Collection(inFamilyVersions))
		proposedVersion := inFamilyVersions[len(inFamilyVersions)-1]
		fmt.Printf("Source: %s, Current pinned Version: %s, Will be upgraded to: %s\n", b.Source, b.Version, proposedVersion.String())
		SetVersionAttribute(b.Block, proposedVersion)
	}
	os.WriteFile(path, hf.Bytes(), 0644)
}
