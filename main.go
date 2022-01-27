package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type ModuleVersionsResponse struct {
	Modules []Module
}

type Module struct {
	Versions []struct {
		Version string `json:"version"`
	}
}

func main() {
	resp, err := http.Get("https://registry.terraform.io/v1/modules/terraform-aws-modules/vpc/aws/versions")
	if err != nil {
		panic(err)
	}
	var data ModuleVersionsResponse

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		panic(readErr)
	}

	jsonErr := json.Unmarshal(body, &data)
	if jsonErr != nil {
		panic(jsonErr)
	}
	fmt.Print(data)
}
