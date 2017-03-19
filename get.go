package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/micromdm/dep"
)

func getResource(args []string) error {
	if len(args) < 1 {
		return errors.New("get requires at least one resource name")
	}

	var run func([]string) error
	switch strings.ToLower(args[0]) {
	case "dep":
		run = getDep
	default:
		usage()
		os.Exit(1)
	}

	if err := run(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	return nil
}

func getDep(args []string) error {
	if len(args) < 1 {
		return errors.New("get dep requires at least one resource name")
	}
	sm := &config{}
	sm.setupBolt()
	client, err := sm.depClient()
	if err != nil {
		return err
	}
	var run func(dep.Client, []string) error
	switch strings.ToLower(args[0]) {
	case "account-info":
		acc, err := client.Account()
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(acc)
		return err
	case "device":
		run = getDepDevice
	case "profile":
		run = getDepProfile
	}

	if err := run(client, args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	return nil
}

func getDepProfile(client dep.Client, args []string) error {
	flagset := flag.NewFlagSet("profile", flag.ExitOnError)
	var (
		flUUID = flagset.String("uuid", "", "profile uuid")
	)
	if err := flagset.Parse(args); err != nil {
		return err
	}

	profile, err := client.FetchProfile(*flUUID)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err = enc.Encode(profile)
	return err
}

func getDepDevice(client dep.Client, args []string) error {
	flagset := flag.NewFlagSet("device", flag.ExitOnError)
	var (
		flSerial = flagset.String("serial", "", "device serial number")
	)
	if err := flagset.Parse(args); err != nil {
		return err
	}

	resp, err := client.DeviceDetails([]string{*flSerial})
	if err != nil {
		return err
	}
	if dev, ok := resp.Devices[*flSerial]; ok && dev.SerialNumber != "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(dev)
		return err
	} else {
		return errors.New("no device information")
	}
	return nil
}
