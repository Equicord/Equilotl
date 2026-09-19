/*
 * SPDX-License-Identifier: GPL-3.0
 * Vencord Installer, a cross platform gui/cli app for installing Vencord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

import (
	"equilotl/buildinfo"
	"image/color"
)

const ReleaseUrl = "https://api.github.com/repos/Equicord/Equicord/releases/latest"
const ReleaseUrlFallback = "https://equicord.org/releases/equicord"
const InstallerReleaseUrl = "https://api.github.com/repos/Equicord/Equilotl/releases/latest"
const InstallerReleaseUrlFallback = "https://equicord.org/releases/equilotl"

var UserAgent = "Equilotl/" + buildinfo.InstallerGitHash + " (https://github.com/Equicord/Equilotl)"

var (
	DiscordGreen  = color.RGBA{R: 0x2D, G: 0x7C, B: 0x46, A: 0xFF}
	DiscordRed    = color.RGBA{R: 0xEC, G: 0x41, B: 0x44, A: 0xFF}
	DiscordBlue   = color.RGBA{R: 0x58, G: 0x65, B: 0xF2, A: 0xFF}
	DiscordYellow = color.RGBA{R: 0xfe, G: 0xe7, B: 0x5c, A: 0xff}
)

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