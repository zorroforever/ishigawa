package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/micromdm/dep"
	"github.com/pkg/errors"
)

func applyResource(args []string) error {
	if len(args) < 1 {
		return errors.New("apply requires at least one resource name")
	}

	sm := &config{}
	sm.setupBolt()
	client, err := sm.depClient()
	if err != nil {
		return err
	}

	var run func(dep.Client, []string) error
	switch strings.ToLower(args[0]) {
	case "dep-profile":
		run = defineDEPProfile
	default:
		usage()
		os.Exit(1)
	}

	if err := run(client, args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	return nil
}

func defineDEPProfile(client dep.Client, args []string) error {
	flagset := flag.NewFlagSet("dep-profile", flag.ExitOnError)
	var (
		flPath = flagset.String("f", "", "path to dep profile JSON")
	)
	if err := flagset.Parse(args); err != nil {
		return err
	}

	if *flPath == "" {
		return errors.New("must specify a path to a profile json")
	}

	data, err := ioutil.ReadFile(*flPath)
	if err != nil {
		return errors.Wrapf(err, "reading DEP profile JSON file: %s", *flPath)
	}

	var profile dep.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return err
	}

	resp, err := client.DefineProfile(&profile)
	if err != nil {
		return err
	}
	fmt.Printf("updated profile %s.\n", resp.ProfileUUID)
	return nil
}
