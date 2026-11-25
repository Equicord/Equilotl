/*
 * SPDX-License-Identifier: GPL-3.0
 * Vencord Installer, a cross platform gui/cli app for installing Vencord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

import (
	"equilotl/buildinfo"
)

const ReleaseUrl = "https://api.github.com/repos/Equicord/Equicord/releases/latest"
const ReleaseUrlFallback = "https://equicord.org/releases/equicord"
const InstallerReleaseUrl = "https://api.github.com/repos/Equicord/Equilotl/releases/latest"
const InstallerReleaseUrlFallback = "https://equicord.org/releases/equilotl"

var UserAgent = "Equilotl/" + buildinfo.InstallerGitHash + " (https://github.com/Equicord/Equilotl)"

const SupportUrl = "https://equicord.org/discord"

var LinuxDiscordNames = []string{
	"Discord",
	"DiscordPTB",
	"DiscordCanary",
	"DiscordDevelopment",
	"discord",
	"discordptb",
	"discordcanary",
	"discorddevelopment",
	"discord-ptb",
	"discord-canary",
	"discord-development",
	// Flatpak
	"com.discordapp.Discord",
	"com.discordapp.DiscordPTB",
	"com.discordapp.DiscordCanary",
	"com.discordapp.DiscordDevelopment",
}
