/*
Copyright © 2022 - 2023 SUSE LLC

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

package cmd

import (
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/mount-utils"

	"github.com/rancher/elemental-cli/cmd/config"
	"github.com/rancher/elemental-cli/pkg/action"
	elementalError "github.com/rancher/elemental-cli/pkg/error"
)

func MountRootFSCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "mount-rootfs MOUNTPOINT",
		Short: "Mount rootfs",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := exec.LookPath("mount")
			if err != nil {
				return err
			}
			mounter := mount.New(path)

			cfg, err := config.ReadConfigRun(viper.GetString("config-dir"), cmd.Flags(), mounter)
			if err != nil {
				cfg.Logger.Errorf("Error reading config: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingRunConfig)
			}

			flags := cmd.Flags()
			s, err := config.ReadMountSpec(cfg, flags)
			if err != nil {
				cfg.Logger.Errorf("Error reading spec: %s\n", err)
				return elementalError.NewFromError(err, elementalError.ReadingSpecConfig)
			}

			mount := action.NewMountRootFS(s, cfg)
			return mount.MountRootFS()
		},
	}
	root.AddCommand(c)
	c.Flags().String("image", "/cOS/active.img", "Image to mount as root, relative to state root")
	c.Flags().String("mountpoint", "/sysroot", "Mountpoint for rootfs")
	c.Flags().String("root-perm", "ro", "Permissions for root mount")
	c.Flags().Bool("switch-root", true, "Switch into newly mounted root")
	c.Flags().StringArray("volumes", []string{"LABEL=COS_OEM:/oem", "LABEL=COS_PERSISTENT:/usr/local"}, "")
	c.Flags().String("overlay", "tmpfs:25%", "")
	c.Flags().StringArray("rw-paths", []string{"/var", "/etc"}, "")
	c.Flags().StringArray("persistent-state-paths", []string{"/etc", "/root", "/home", "/opt", "/usr/local", "/var"}, "")
	c.Flags().Bool("persistent-state-bind", true, "")
	return c
}

// register the subcommand into rootCmd
var _ = MountRootFSCmd(rootCmd)
