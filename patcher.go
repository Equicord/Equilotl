/*
 * SPDX-License-Identifier: GPL-3.0
 * Vencord Installer, a cross platform gui/cli app for installing Vencord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	path "path/filepath"
	"strings"

	"github.com/ProtonMail/go-appdir"
)

var BaseDir string
var BaseDirErr error
var EquicordDirectory string

var ErrAlreadyReported = errors.New("already reported")

func init() {
	if dir := os.Getenv("EQUICORD_USER_DATA_DIR"); dir != "" {
		Log.Debug("Using EQUICORD_USER_DATA_DIR")
		BaseDir = dir
	} else if dir = os.Getenv("DISCORD_USER_DATA_DIR"); dir != "" {
		Log.Debug("Using DISCORD_USER_DATA_DIR/../EquicordData")
		BaseDir = path.Join(dir, "..", "EquicordData")
	} else {
		Log.Debug("Using UserConfig")
		BaseDir = appdir.New("Equicord").UserConfig()
	}
	dir := os.Getenv("EQUICORD_DIRECTORY")
	if dir == "" {
		if !ExistsFile(BaseDir) {
			BaseDirErr = os.Mkdir(BaseDir, 0755)
			if BaseDirErr != nil {
				Log.Error("Failed to create", BaseDir, BaseDirErr)
			} else {
				BaseDirErr = FixOwnership(BaseDir)
			}
		}
	}
	if dir != "" {
		Log.Debug("Using EQUICORD_DIRECTORY")
		EquicordDirectory = dir
	} else {
		EquicordDirectory = path.Join(BaseDir, "equicord.asar")
	}
}

type DiscordInstall struct {
	path          string // the base path
	branch        string // canary / stable / ...
	resourcesPath string // app-{VERSION}/resources
	isPatched     bool
	isFlatpak     bool
	isOpenAsar    *bool
}

//region Patch

func patchAppAsar(dir string) (err error) {
	appAsar := path.Join(dir, "app.asar")
	_appAsar := path.Join(dir, "_app.asar")

	var renamesDone [][]string
	defer func() {
		if err != nil && len(renamesDone) > 0 {
			Log.Error("Failed to patch. Undoing partial patch")
			for _, rename := range renamesDone {
				if innerErr := os.Rename(rename[1], rename[0]); innerErr != nil {
					Log.Error("Failed to undo partial patch. This install is probably bricked.", innerErr)
				} else {
					Log.Info("Successfully undid all changes")
				}
			}
		}
	}()

	Log.Debug("Renaming", appAsar, "to", _appAsar)
	if err := os.Rename(appAsar, _appAsar); err != nil {
		err = CheckIfErrIsCauseItsBusyRn(err)
		Log.Error(err.Error())
		return err
	}
	renamesDone = append(renamesDone, []string{appAsar, _appAsar})

	Log.Debug("Writing custom app.asar to", appAsar)
	if err := WriteAppAsar(appAsar, EquicordDirectory); err != nil {
		return err
	}

	return nil
}

func (di *DiscordInstall) patch() error {
	Log.Info("Patching " + di.path + "...")
	if LatestHash != InstalledHash {
		if err := InstallLatestBuilds(); err != nil {
			return ErrAlreadyReported
		}
	}

	PreparePatch(di)

	if di.isPatched {
		Log.Info(di.path, "is already patched. Unpatching first...")
		if err := di.unpatch(); err != nil {
			if errors.Is(err, os.ErrPermission) {
				return err
			}
			return errors.New("patch: Failed to unpatch already patched install '" + di.path + "':\n" + err.Error())
		}
	}

	if err := patchAppAsar(di.resourcesPath); err != nil {
		return err
	}

	Log.Info("Successfully patched", di.path)
	di.isPatched = true

	if di.isFlatpak {
		pathElements := strings.Split(di.path, "/")
		var name string
		for _, e := range pathElements {
			if strings.HasPrefix(e, "com.discordapp") {
				name = e
				break
			}
		}

		Log.Debug("This is a flatpak. Trying to grant the Flatpak access to", EquicordDirectory+"...")

		isSystemFlatpak := strings.HasPrefix(di.path, "/var")
		var args []string
		if !isSystemFlatpak {
			args = append(args, "--user")
		}
		args = append(args, "override", name, "--filesystem="+EquicordDirectory)
		fullCmd := "flatpak " + strings.Join(args, " ")

		Log.Debug("Running", fullCmd)

		var err error
		if !isSystemFlatpak && os.Getuid() == 0 {
			// We are operating on a user flatpak but are root
			actualUser := os.Getenv("SUDO_USER")
			Log.Debug("This is a user install but we are root. Using su to run as", actualUser)
			cmd := exec.Command("su", "-", actualUser, "-c", "sh", "-c", fullCmd)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		} else {
			cmd := exec.Command("flatpak", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		}
		if err != nil {
			return errors.New("Failed to grant Discord Flatpak access to " + EquicordDirectory + ": " + err.Error())
		}
	}
	return nil
}

//endregion

// region Unpatch

func isEquicordLoaderAppAsar(appAsar string) (bool, error) {
	stat, err := os.Stat(appAsar)
	if err != nil {
		return false, err
	}
	if stat.Size() > 128*1024 {
		return false, nil
	}
	b, err := os.ReadFile(appAsar)
	if err != nil {
		return false, err
	}
	return bytes.Contains(b, []byte(PackageJson)) && bytes.Contains(b, []byte("require(")), nil
}

func cleanupDesyncedPatchedInstall(dir string) (bool, error) {
	appAsar := path.Join(dir, "app.asar")
	_appAsar := path.Join(dir, "_app.asar")

	isLoader, err := isEquicordLoaderAppAsar(appAsar)
	if err != nil {
		return false, err
	}
	if isLoader {
		return false, nil
	}

	Log.Warn("Detected a patched install with a non-Equicord app.asar. Discord was likely updated while patched; removing stale _app.asar")

	if err = os.Remove(_appAsar); err != nil {
		return false, CheckIfErrIsCauseItsBusyRn(err)
	}
	return true, nil
}

func unpatchAppAsar(dir string) (errOut error) {
	appAsar := path.Join(dir, "app.asar")
	appAsarTmp := path.Join(dir, "app.asar.tmp")
	_appAsar := path.Join(dir, "_app.asar")

	cleanedUp, err := cleanupDesyncedPatchedInstall(dir)
	if err != nil {
		return err
	}
	if cleanedUp {
		return nil
	}

	var renamesDone [][]string
	defer func() {
		if errOut != nil && len(renamesDone) > 0 {
			Log.Error("Failed to unpatch. Undoing partial unpatch")
			for _, rename := range renamesDone {
				if innerErr := os.Rename(rename[1], rename[0]); innerErr != nil {
					Log.Error("Failed to undo partial unpatch. This install is probably bricked.", innerErr)
				} else {
					Log.Info("Successfully undid all changes")
				}
			}
		} else if errOut == nil {
			if innerErr := os.RemoveAll(appAsarTmp); innerErr != nil {
				Log.Warn("Failed to delete temporary app.asar (patch folder) backup. This is whatever but you might want to delete it manually.", innerErr)
			}
		}
	}()

	Log.Debug("Deleting", appAsar)
	if err := os.Rename(appAsar, appAsarTmp); err != nil {
		err = CheckIfErrIsCauseItsBusyRn(err)
		Log.Error(err.Error())
		errOut = err
		return
	}
	renamesDone = append(renamesDone, []string{appAsar, appAsarTmp})

	Log.Debug("Renaming", _appAsar, "to", appAsar)
	if err := os.Rename(_appAsar, appAsar); err != nil {
		err = CheckIfErrIsCauseItsBusyRn(err)
		Log.Error(err.Error())
		errOut = err
		return
	}
	renamesDone = append(renamesDone, []string{_appAsar, appAsar})
	return
}

func (di *DiscordInstall) unpatch() error {
	Log.Info("Unpatching " + di.path + "...")

	PreparePatch(di)

	if err := unpatchAppAsar(di.resourcesPath); err != nil {
		return err
	}

	Log.Info("Successfully unpatched", di.path)
	di.isPatched = false
	return nil
}

//endregion
