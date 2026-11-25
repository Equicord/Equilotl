/*
 * SPDX-License-Identifier: GPL-3.0
 * Vencord Installer, a cross platform gui/cli app for installing Vencord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

import (
	"os"
	path "path/filepath"
	"strings"
)

var macosNames = map[string]string{
	"stable": "Discord.app",
	"ptb":    "Discord PTB.app",
	"canary": "Discord Canary.app",
	"dev":    "Discord Development.app",
}

func ParseDiscordNew(p, branch string, isFlatpak bool) *DiscordInstall {
	return nil
}

func ParseDiscord(p, branch string) *DiscordInstall {
	if !ExistsFile(p) {
		return nil
	}

	resources := path.Join(p, "/Contents/Resources")
	if !ExistsFile(resources) {
		return nil
	}

	isPatched := isResourcesFolderPatched(resources)
	if !isPatched && !isResourcesFolderValid(resources) {
		return nil
	}

	if branch == "" {
		branch = GetBranch(strings.TrimSuffix(p, ".app"))
	}

	return &DiscordInstall{
		path:          p,
		branch:        branch,
		resourcesPath: resources,
		isPatched:     isPatched,
		isFlatpak:     false,
	}
}

func FindDiscords() []any {
	var discords []any
	bases := []string{
		"/Applications",
		path.Join(os.Getenv("HOME"), "Applications"),
	}
	supportBase := path.Join(os.Getenv("HOME"), "Library", "Application Support")

	for _, branch := range []string{"stable", "ptb", "canary", "dev"} {
		dirname := macosNames[branch]

		branchDir := "discord" + branch
		if branch == "stable" {
			branchDir = "discord"
		} else if branch == "dev" {
			branchDir = "discorddevelopment"
		}

		matches, err := path.Glob(path.Join(supportBase, branchDir, "app-*", dirname))
		if err == nil {
			var latest *DiscordInstall
			var latestVer []int
			for _, match := range matches {
				ver := ParseAppVersion(path.Base(path.Dir(match)))
				if ver == nil {
					continue
				}
				if discord := ParseDiscord(match, branch); discord != nil && (latest == nil || CompareAppVersion(ver, latestVer) > 0) {
					latest, latestVer = discord, ver
				}
			}
			if latest != nil {
				Log.Debug("Found Discord Install at", latest.path)
				discords = append(discords, latest)
			}
		}

		for _, base := range bases {
			p := path.Join(base, dirname)
			if discord := ParseDiscord(p, branch); discord != nil {
				Log.Debug("Found Discord Install at", p)
				discords = append(discords, discord)
			}
		}
	}
	return discords
}

func PreparePatch(di *DiscordInstall) {}

func FixOwnership(_ string) error {
	return nil
}

func CheckScuffedInstall() bool {
	return false
}
