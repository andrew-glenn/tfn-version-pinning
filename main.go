package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/Masterminds/semver"
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

func main() {
	resp, err := http.Get("https://registry.terraform.io/v1/modules/terraform-aws-modules/vpc/aws/versions")
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

	sample_version, err := semver.NewVersion("2.0.0")
	if err != nil {
		panic(err)
	}

	var inFamilyVersions []*semver.Version
	for _, ver := range data.Versions {
		sv, err := semver.NewVersion(ver.Version)
		if err != nil {
			panic(err)
		}
		if sample_version.Major() == sv.Major() {
			inFamilyVersions = append(inFamilyVersions, sv)
		}
	}
	sort.Sort(semver.Collection(inFamilyVersions))
	fmt.Printf("Version [%s] for module [%s] can be bumped to [%s]", sample_version.String(), data.Source, inFamilyVersions[len(inFamilyVersions)-1].String())
	// TODO: Cross reference provider requirements. 
	// TODO: interpolate modules from terraform files.
