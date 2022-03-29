package main

import "github.com/hashicorp/hcl/v2/hclwrite"

import (
	"errors"
	"flag"
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"io/ioutil"
	"net/http"
	"os"
	"sort"

	"github.com/zclconf/go-cty/cty"

	"path/filepath"

	"github.com/Masterminds/semver"
	"github.com/hashicorp/hcl/v2/hclsyntax"
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
	Module    []HCLModule   `hcl:"module,block"`
	Providers []HCLProvider `hcl:"provider,block"`
}

type HCLProvider struct {
	Name      string
	UserAgent ProviderUserAgent
	Body      *hclwrite.Body
}

type ProviderUserAgent struct {
	ProductName    TokenSurgeryReference
	ProductVersion TokenSurgeryReference
	Comment        TokenSurgeryReference
	og_tokens      hclwrite.Tokens
}

func (m *ProviderUserAgent) GenerateNewTokens() hclwrite.Tokens {
	var modified hclwrite.Tokens
	for idx, token := range m.og_tokens {
		switch idx {
		case m.ProductName.Index:
			modified = append(modified, m.ProductName.Token)
		case m.ProductVersion.Index:
			modified = append(modified, m.ProductVersion.Token)
		case m.Comment.Index:
			modified = append(modified, m.Comment.Token)
		default:
			modified = append(modified, token)
		}
	}
	return modified
}

type TokenSurgeryReference struct {
	Index int
	Token *hclwrite.Token
	Value string
}

func (tsr *TokenSurgeryReference) NewValue(s string) {
	tsr.Value = s
	tsr.Token.Bytes = []byte(s)
}

func UAFromTokens(tokens hclwrite.Tokens) ProviderUserAgent {
	var ident bool = false
	var IdentToken *hclwrite.Token
	token_indents := make(map[string]TokenSurgeryReference)
	for idx, t := range tokens {
		if t.Type == hclsyntax.TokenIdent {
			ident = true
			IdentToken = t
			continue
		}
		if (t.Type == hclsyntax.TokenQuotedLit) && (ident == true) {
			ident_str := string(IdentToken.Bytes)
			value_str := string(t.Bytes)
			token_indents[ident_str] = TokenSurgeryReference{
				Index: idx,
				Token: t,
				Value: value_str,
			}
			ident = false
		}
	}
	ua := ProviderUserAgent{
		ProductName:    token_indents["product_name"],
		ProductVersion: token_indents["product_version"],
		Comment:        token_indents["comment"],
		og_tokens:      tokens,
	}
	return ua
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

func CreateProvider(block *hclwrite.Block) (HCLProvider, error) {
	provider := block.Labels()[0]
	user_agent := block.Body().GetAttribute("user_agent")
	if user_agent == nil {
		return HCLProvider{}, errors.New("No useragent")
	}
	ff := UAFromTokens(user_agent.BuildTokens(nil))
	return HCLProvider{
		Name:      provider,
		UserAgent: ff,
		Body:      block.Body(),
	}, nil
}

func WalkMatch(root string, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
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
	cwd, _ := os.Getwd()
	flag.StringVar(&path, "path", cwd, "path to terraform module")
	flag.Parse()
	files, err := WalkMatch(path, "*.tf")
	if err != nil {
		panic(err)
	}
	VersionFileSemver, _ := MostRecentVersionInVersionFile(path)
	RepoSemVer := MostRecentSemVerForRepo(path)
	for _, path := range files {
		var changes bool
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		var modules []HCLModule
		var providers []HCLProvider

		hf, _ := hclwrite.ParseConfig(data, path, hcl.InitialPos)

		for _, v := range hf.Body().Blocks() {
			if v.Type() == "module" {
				modules = append(modules, CreateModule(v))
			}
			if v.Type() == "provider" {
				ua := v.Body().GetAttribute("user_agent")
				if ua != nil {
					pv, err := CreateProvider(v)
					if err == nil {
						providers = append(providers, pv)
					}
				}
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
			if currentVersion == proposedVersion {
				continue
			}
			fmt.Printf("Source: %s, Current pinned Version: %s, Will be upgraded to: %s\n", b.Source, b.Version, proposedVersion.String())
			changes = true
			SetVersionAttribute(b.Block, proposedVersion)
		}

		for _, m := range providers {
			if m.Name == "awscc" {
				if VersionFileSemver == nil {
					m.UserAgent.ProductVersion.NewValue(RepoSemVer.String())
				} else {
					semvers := []*semver.Version{VersionFileSemver, RepoSemVer}
					sort.Sort(semver.Collection(semvers))
					m.UserAgent.ProductVersion.NewValue(semvers[len(semvers)-1].String())
				}
				NewTokens := m.UserAgent.GenerateNewTokens()
				m.Body.SetAttributeRaw("user_agent", NewTokens[2:])
				changes = true
			}
		}
		if changes {
			os.WriteFile(path, hclwrite.Format(hf.Bytes()), 0644)
		}
	}
}
