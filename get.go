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
		return errors.New("invalid dep resource name")
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
	case "profile-template":
		run = getDepProfileTpl
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

func getDepProfileTpl(client dep.Client, args []string) error {
	/* omitempty flags on the Profile struct are preventing the (Apple-
	   defined) false defaults from showing up in the example.

	p := dep.Profile{
		ProfileName:           "(Required) Human readable name",
		URL:                   "https://mymdm.example.org",
		AllowPairing:          true,
		IsSupervised:          false,
		IsMultiUser:           false,
		IsMandatory:           false,
		AwaitDeviceConfigured: false,
		IsMDMRemovable:        true,
		SupportPhoneNumber:    "(Optional) +1 408 555 1010",
		SupportEmailAddress:   "(Optional) support@example.com",
		OrgMagic:              "(Optional)",
		AnchorCerts:           []string{},
		SupervisingHostCerts:  []string{},
		SkipSetupItems:        []string{},
		Department:            "(Optional) support@example.com",
		Devices:               []string{},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err := enc.Encode(&p)
	*/

	resp := `{
  "profile_name": "(Required) Human readable name",
  "url": "https://mymdm.example.org",
  "allow_pairing": true,
  "is_supervised": false,
  "is_multi_user": false,
  "is_mandatory": false,
  "await_device_configured": false,
  "is_mdm_removable": true,
  "support_phone_number": "(Optional) +1 408 555 1010",
  "support_email_address": "(Optional) support@example.com",
  "org_magic": "(Optional)",
  "anchor_certs": [],
  "supervising_host_certs": [],
  "skip_setup_items": [],
  "deparment": "(Optional) support@example.com",
  "devices": []
}`
	fmt.Println(resp)
	return nil
}
