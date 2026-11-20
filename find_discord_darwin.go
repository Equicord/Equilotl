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

func CompareAppVersion(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

func ParseAppVersion(name string) []int {
	rest, ok := strings.CutPrefix(name, "app-")
	if !ok {
		return nil
	}

	parts := strings.Split(rest, ".")
	version := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		version = append(version, n)
	}
	return version
}

func FindDiscords() []any {
	var discords []any
	home := os.Getenv("HOME")

	bases := []string{
		"/Applications",
		path.Join(home, "Applications"),
	}

	appSupportNames := map[string]string{
		"stable": "discord",
		"ptb":    "discordptb",
		"canary": "discordcanary",
		"dev":    "discorddevelopment",
	}

	for branch, appName := range macosNames {
		for _, base := range bases {
			p := path.Join(base, appName)
			if discord := ParseDiscord(p, branch); discord != nil {
				Log.Debug("Found Discord Install at", p)
				discords = append(discords, discord)
			}
		}

		appSupport := path.Join(
			home,
			"Library",
			"Application Support",
			appSupportNames[branch],
		)

		entries, err := os.ReadDir(appSupport)
		if err != nil {
			continue
		}

		var latestVersion []int
		var latestDiscord *DiscordInstall

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			version := ParseAppVersion(entry.Name())
			if version == nil {
				continue
			}

			p := path.Join(appSupport, entry.Name(), appName)
			discord := ParseDiscord(p, branch)
			if discord == nil {
				continue
			}

			if latestDiscord == nil || CompareAppVersion(version, latestVersion) > 0 {
				latestVersion = version
				latestDiscord = discord
			}
		}

		if latestDiscord != nil {
			Log.Debug("Found versioned Discord Install at", latestDiscord.path)
			discords = append(discords, latestDiscord)
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
