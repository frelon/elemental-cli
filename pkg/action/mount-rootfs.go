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

package action

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rancher/elemental-cli/pkg/constants"
	"github.com/rancher/elemental-cli/pkg/elemental"
	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
	"github.com/rancher/elemental-cli/pkg/utils"
)

type MountRootFSAction struct {
	cfg  *v1.RunConfig
	spec *v1.MountSpec
	e    *elemental.Elemental
}

func NewMountRootFS(spec *v1.MountSpec, cfg *v1.RunConfig) *MountRootFSAction {
	return &MountRootFSAction{
		cfg:  cfg,
		spec: spec,
		e:    elemental.NewElemental(&cfg.Config),
	}
}

// MountRootFS mounts the rootfs in several stages.
func (m MountRootFSAction) MountRootFS() error {
	utils.MkdirAll(m.cfg.Config.Fs, constants.OverlayDir, constants.DirPerm)

	// mount overlay base
	if err := m.cfg.Config.Mounter.Mount(constants.TmpFs, constants.OverlayDir, constants.TmpFs, []string{"defaults", "size=25%"}); err != nil {
		m.cfg.Config.Logger.Errorf("Error mounting overlay: %v", err.Error())
		return err
	}

	fstab := []string{
		"/dev/loop0\t/\tauto\tro\t0\t0",
		fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%d", constants.TmpFs, constants.OverlayDir, constants.TmpFs, "defaults,size=25%", 0, 0),
	}

	// mount overlay
	for _, rw := range m.spec.RwPaths {
		trimmed := strings.TrimPrefix(rw, "/")
		upper := filepath.Join(constants.OverlayDir, strings.ReplaceAll(trimmed, "/", "-")+".overlay", "upper")
		work := filepath.Join(constants.OverlayDir, strings.ReplaceAll(trimmed, "/", "-")+".overlay", "work")
		merged := filepath.Join(m.spec.MountPoint, rw)

		if err := utils.MkdirAll(m.cfg.Config.Fs, upper, constants.DirPerm); err != nil {
			m.cfg.Config.Logger.Errorf("Error mkdir upper: %v", err.Error())
			return err
		}

		if err := utils.MkdirAll(m.cfg.Config.Fs, work, constants.DirPerm); err != nil {
			m.cfg.Config.Logger.Errorf("Error mkdir work: %v", err.Error())
			return err
		}

		if err := utils.MkdirAll(m.cfg.Config.Fs, merged, constants.DirPerm); err != nil {
			m.cfg.Config.Logger.Errorf("Error mkdir merged: %v", err.Error())
			return err
		}

		mount := NewOverlayMount(rw, m.spec.MountPoint, constants.OverlayDir, constants.OverlayFs)
		if err := mount.Mount(&m.cfg.Config); err != nil {
			m.cfg.Config.Logger.Errorf("Error mounting overlay: %v", err.Error())
			return err
		}

		fstab = append(fstab, mount.FstabLine())
	}

	// mount -t auto "${mount#*:}" "/sysroot${mount%%:*}"
	// echo "${mount#*:} ${mount%%:*} auto defaults 0 0\n"

	// mount volumes/persistent

	for _, volume := range m.spec.Volumes {
		opts := []string{"defaults"}

		mountpoint := filepath.Join(m.spec.MountPoint, volume.MountPoint)
		if err := utils.MkdirAll(m.cfg.Config.Fs, mountpoint, constants.DirPerm); err != nil {
			m.cfg.Config.Logger.Errorf("Error mkdir merged: %v", err.Error())
			return err
		}

		disk := filepath.Join("/dev/disk/by-label", volume.Label)
		if err := m.cfg.Mounter.Mount(disk, mountpoint, constants.AutoFs, opts); err != nil {
			m.cfg.Config.Logger.Errorf("Error mounting overlay: %v", err.Error())
			return err
		}

		line := fmt.Sprintf("%s\t%s\t%s\t%s", disk, volume.MountPoint, constants.AutoFs, strings.Join(opts, ","))

		fstab = append(fstab, line)
	}

	// mount state

	if err := utils.MkdirAll(m.cfg.Config.Fs, m.spec.PersistentStateTarget, constants.DirPerm); err != nil {
		m.cfg.Config.Logger.Errorf("Error mkdir base: %v", err.Error())
		return err
	}

	for _, statePath := range m.spec.PersistentStatePaths {
		base := filepath.Join(m.spec.MountPoint, statePath)
		if err := utils.MkdirAll(m.cfg.Config.Fs, base, constants.DirPerm); err != nil {
			m.cfg.Config.Logger.Errorf("Error mkdir base: %v", err.Error())
			return err
		}

		trimmed := strings.TrimPrefix(statePath, "/")
		stateDir := filepath.Join(m.spec.MountPoint, strings.ReplaceAll(trimmed, "/", "-")+".bind")
		if err := utils.MkdirAll(m.cfg.Config.Fs, stateDir, constants.DirPerm); err != nil {
			m.cfg.Config.Logger.Errorf("Error mkdir base: %v", err.Error())
			return err
		}

		// TODO: old immutable-rootfs rsyncs stuff here...
		// TODO: should create OverlayMount if PersistentStateBind is set to false
		mount := NewBindMount(stateDir, m.spec.MountPoint, m.spec.PersistentStateTarget)

		if err := mount.Mount(&m.cfg.Config); err != nil {
			m.cfg.Config.Logger.Errorf("Error writing fstab: %v", err.Error())
			return err
		}

		fstab = append(fstab, mount.FstabLine())
	}

	fstabPath := filepath.Join(m.spec.MountPoint, "etc", "fstab")
	if err := m.cfg.Config.Fs.WriteFile(fstabPath, []byte(strings.Join(fstab, "\n")), 0644); err != nil {
		m.cfg.Config.Logger.Errorf("Error writing fstab: %v", err.Error())
		return err
	}

	m.cfg.Logger.Infof("RootFS mounted, ready for switching root.")
	return nil
}

type OverlayMount struct {
	path       string
	base       string
	overlayDir string
	source     string
	mountTo    string
}

func NewOverlayMount(path, base, overlayDir, source string) OverlayMount {
	return OverlayMount{
		path:       path,
		base:       base,
		overlayDir: overlayDir,
		source:     source,
	}
}

func (m OverlayMount) FstabLine() string {
	trimmed := strings.TrimPrefix(m.path, "/")
	upper := filepath.Join(m.overlayDir, strings.ReplaceAll(trimmed, "/", "-")+".overlay", "upper")
	work := filepath.Join(m.overlayDir, strings.ReplaceAll(trimmed, "/", "-")+".overlay", "work")
	fstabOpts := []string{"defaults", fmt.Sprintf("lowerdir=%s", m.source), fmt.Sprintf("upperdir=%s", upper), fmt.Sprintf("workdir=%s", work)}
	return fmt.Sprintf("%s\t%s\t%s\t%s", m.source, m.path, constants.OverlayFs, strings.Join(fstabOpts, ","))
}

func (m OverlayMount) Mount(cfg *v1.Config) error {
	trimmed := strings.TrimPrefix(m.path, "/")
	upper := filepath.Join(m.overlayDir, strings.ReplaceAll(trimmed, "/", "-")+".overlay", "upper")
	work := filepath.Join(m.overlayDir, strings.ReplaceAll(trimmed, "/", "-")+".overlay", "work")
	opts := []string{"defaults", fmt.Sprintf("lowerdir=%s", filepath.Join(m.base, m.path)), fmt.Sprintf("upperdir=%s", upper), fmt.Sprintf("workdir=%s", work)}
	return cfg.Mounter.Mount(m.source, filepath.Join(m.base, m.path), constants.OverlayFs, opts)
}

type BindMount struct {
	path     string
	base     string
	stateDir string
}

// example:
// /usr/local/.state/etc-systemd.bind /etc/systemd none defaults,bind 0 0

// state_dir="/sysroot${state_target}/${mount//\//-}.bind"
// mount="${mount#/}"
// base="/sysroot/${mount}"
// state_dir="/sysroot${state_target}/${mount//\//-}.bind"

// rsync -aqAX "${base}/" "${state_dir}/"
// mount -o defaults,bind "${state_dir}" "${base}"
// fstab_line="${state_dir##/sysroot} /${mount} none defaults,bind 0 0\n"

func NewBindMount(path, base, stateDir string) BindMount {
	return BindMount{
		path:     path,
		base:     base,
		stateDir: stateDir,
	}
}

func (m BindMount) FstabLine() string {
	fstabOpts := []string{"defaults", "bind"}
	return fmt.Sprintf("%s\t%s\t%s\t%s", m.stateDir, m.path, constants.NoneFs, strings.Join(fstabOpts, ","))
}

func (m BindMount) Mount(cfg *v1.Config) error {
	source := filepath.Join(m.base, m.path)
	opts := []string{"defaults", "bind"}
	return cfg.Mounter.Mount(source, m.stateDir, constants.NoneFs, opts)
}
