package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/micromdm/nano/device"
)

// TODO this is a temporary command to get some stuff working
// we need to remove this and add more robust/general "list/describe" commands
func dev(args []string) error {
	flagset := flag.NewFlagSet("devinfo", flag.ExitOnError)
	var (
		flList     = flagset.Bool("list", false, "list all bucket keys")
		flDescribe = flagset.String("describe", "", "describe a serial")
	)
	flagset.Usage = usageFor(flagset, "micromdm serve [flags]")
	if err := flagset.Parse(args); err != nil {
		return err
	}

	sm := &config{}
	sm.setupBolt()
	db, err := device.NewDB(sm.db, nil)
	if err != nil {
		return err
	}

	if *flDescribe != "" {
		err := describeDevice(db, *flDescribe)
		return err
	}

	if *flList {
		err := listDevices(db)
		return err
	}

	return nil
}

func describeDevice(db *device.DB, serial string) error {
	dev, err := db.DeviceBySerial(serial)
	if err != nil {
		return err
	}
	fmt.Printf("	udid=%s\n", dev.UDID)
	fmt.Printf("	serial=%s\n", dev.SerialNumber)
	fmt.Printf("	prduct_name=%s\n", dev.ProductName)
	return nil
}

func listDevices(db *device.DB) error {
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(device.DeviceBucket))
		if b == nil {
			return errors.New("no device bucket found")
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
