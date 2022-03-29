package main

import (
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Open an existing repository in a specific folder.
func MostRecentSemVerForRepo(path string) *semver.Version {
	// We instantiate a new repository targeting the given path (the .git folder)
	r, _ := git.PlainOpen(path)
	iter, _ := r.Tags()

	var semvers []*semver.Version
	iter.ForEach(func(tag *plumbing.Reference) error {
		nt, _ := semver.NewVersion(strings.Split(string(tag.Name()), "/")[2])
		semvers = append(semvers, nt)
		return nil
	})
	sort.Sort(semver.Collection(semvers))
	return semvers[len(semvers)-1]
}

func MostRecentVersionInVersionFile(path string) (*semver.Version, error) {
	data, err := os.ReadFile(path + "/VERSION")
	if err != nil {
		return nil, errors.New("Unable to open VERSION file")
	}
	ds := string(data)
	nv, err := semver.NewVersion(strings.TrimSuffix(ds, "\n"))
	if err != nil {
		return nil, err
	}
	return nv, nil
}
