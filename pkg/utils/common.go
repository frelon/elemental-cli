/*
Copyright © 2021 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"
	"github.com/rancher-sandbox/elemental-cli/pkg/types/v1"
	"github.com/spf13/afero"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	GPT = "gpt"
	ESP = "esp"
	BIOS = "bios_grub"
	MSDOS = "msdos"
	BOOT = "boot"
)

func GetUrl(client v1.HTTPClient, url string, destination string) error {
	var source io.Reader
	var err error

	switch {
	case strings.HasPrefix(url, "http"), strings.HasPrefix(url, "ftp"), strings.HasPrefix(url, "tftp"):
		fmt.Printf("Downloading from %s to %s\n", url, destination)
		resp, err := client.Get(url)
		if err != nil {return err}
		source = resp.Body
		defer resp.Body.Close()
	default:
		fmt.Printf("Copying from %s to %s\n", url, destination)
		file, err := os.Open(url)
		if err != nil {return err}
		source = file
		defer file.Close()
	}

	dest, err := os.Create(destination)
	defer dest.Close()
	if err != nil {return err}
	nBytes, err := io.Copy(dest, source)
	if err != nil {return err}
	fmt.Printf("Copied %d bytes\n", nBytes)

	return nil
}

func selinuxRelabel(target string, fs afero.Fs) error {
	var err error

	contextFile := fmt.Sprintf("%s/etc/selinux/targeted/contexts/files/file_contexts", target)

	_, err1 := fs.Stat(contextFile)
	contextExists := err1 == nil

	if commandExists("setfiles") && contextExists {
		_, err = exec.Command("setfiles", "-r", target, contextFile, target).CombinedOutput()
	}

	return err
}

func commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func setupStyle(config *v1.RunConfig, fs afero.Fs) {
	var part,boot string

	_, err := fs.Stat("/sys/firmware/efi")
	efiExists := err == nil


	if config.ForceEfi || efiExists {
		part = GPT
		boot = ESP
	} else if config.ForceGpt {
		part = GPT
		boot = BIOS
	} else {
		part = MSDOS
		boot = BOOT
	}

	config.PartTable = part
	config.BootFlag = boot
}

func BootedFrom(label string) bool {
	out, _ := exec.Command("cat",  "/proc/cmdline").CombinedOutput()

	return strings.Contains(string(out), label)
}