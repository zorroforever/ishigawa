package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/micromdm/mdm"
	"github.com/micromdm/micromdm/checkin"
	"github.com/micromdm/micromdm/command"
	"github.com/pkg/errors"
)

const blueprintPath = `/etc/micromdm/blueprint.json`

func hardcodeCommands(sm *config) error {
	if _, err := os.Stat(blueprintPath); os.IsNotExist(err) {
		fmt.Printf("no blueprint specified in %s, skipping default device actions\n", blueprintPath)
		return nil
	}

	bpData, err := ioutil.ReadFile(blueprintPath)
	if err != nil {
		return errors.Wrap(err, "reading blueprint.json file")
	}

	var blueprint Blueprint
	if err := json.Unmarshal(bpData, &blueprint); err != nil {
		return errors.Wrap(err, "decoding blueprint json file")
	}

	sub := sm.pubclient
	cmdsvc := sm.commandService
	pushsvc := sm.pushService
	authEvents, err := sub.Subscribe("hardcode-dep", checkin.AuthenticateTopic)
	if err != nil {
		return errors.Wrapf(err,
			"subscribing devices to %s topic", checkin.AuthenticateTopic)
	}

	go func() {
		for {
			select {
			case event := <-authEvents:
				var ev checkin.Event
				if err := checkin.UnmarshalEvent(event.Message, &ev); err != nil {
					fmt.Println(err)
					continue
				}
				if err := hardcodeList(cmdsvc, &blueprint, ev.Command.UDID); err != nil {
					log.Println(err)
					continue
				}
				go func() {
					time.Sleep(10 * time.Second)
					pushsvc.Push(context.Background(), ev.Command.UDID)
				}()

			}
		}
	}()

	return nil
}

type Blueprint struct {
	ApplicationsURLs []string `json:"install_application_manifest_urls"`
	Profiles         []string `json:"install_profile_mobileconfig_paths"`
}

func hardcodeList(svc command.Service, blueprint *Blueprint, udid string) error {
	ctx := context.Background()
	var requests []*mdm.CommandRequest
	for _, appURL := range blueprint.ApplicationsURLs {
		requests = append(requests, &mdm.CommandRequest{
			RequestType: "InstallApplication",
			UDID:        udid,
			InstallApplication: mdm.InstallApplication{
				ManifestURL:     appURL,
				ManagementFlags: 1,
			},
		})
	}

	for _, profilePath := range blueprint.Profiles {
		profilePayload, err := ioutil.ReadFile(profilePath)
		if err != nil {
			fmt.Printf("error reading profile %s\n, err: %s ", profilePath, err)
		}
		requests = append(requests, &mdm.CommandRequest{
			RequestType: "InstallProfile",
			UDID:        udid,
			InstallProfile: mdm.InstallProfile{
				Payload: profilePayload,
			},
		})
	}

	devConfigured := &mdm.CommandRequest{
		RequestType: "DeviceConfigured",
		UDID:        udid,
	}
	requests = append(requests, devConfigured)

	for _, r := range requests {
		_, err := svc.NewCommand(ctx, r)
		if err != nil {
			return err
		}
	}
	return nil
}
