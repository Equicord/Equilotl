//go:build !cli

/*
 * SPDX-License-Identifier: GPL-3.0
 * Vencord Installer, a cross platform gui/cli app for installing Vencord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

// typedef struct GLFWwindow GLFWwindow;
// const char *glfwGetKeyName(int key, int scancode);
// int glfwGetPlatform(void);
// int glfwInit(void);
// GLFWwindow *glfwGetCurrentContext(void);
// void glfwGetCursorPos(GLFWwindow *window, double *xpos, double *ypos);
// int glfwGetMouseButton(GLFWwindow *window, int button);
// int glfwGetWindowAttrib(GLFWwindow *window, int attrib);
// void glfwIconifyWindow(GLFWwindow *window);
// void glfwMaximizeWindow(GLFWwindow *window);
// void glfwRestoreWindow(GLFWwindow *window);
// enum { GLFW_PLATFORM_WAYLAND = 0x00060003, GLFW_MAXIMIZED = 0x00020008 };
import "C"

import (
	"bytes"
	_ "embed"
	"equilotl/buildinfo"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	path "path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	g "github.com/AllenDang/giu"
	"golang.org/x/image/draw"

	// png decoder for icon
	_ "image/png"
)

const (
	EquicordRepoUrl  = "https://github.com/Equicord/Equicord"
	InstallerRepoUrl = "https://github.com/Equicord/Equilotl"
)

var (
	discords        []any
	radioIdx        int
	customChoiceIdx int

	customDir              string
	autoCompleteDir        string
	autoCompleteFile       string
	autoCompleteCandidates []string
	autoCompleteIdx        int
	lastAutoComplete       string
	didAutoComplete        bool

	modalId          = 0
	modalTitle       = "Oh No :("
	modalMessage     = "You should never see this"
	modalExtra       = ""
	modalSupportLink = ""

	acceptedOpenAsar   bool
	showedUpdatePrompt bool
	showCustomLocation bool

	githubDone atomic.Bool
	appIcon    image.Image

	appIconTex     *g.Texture
	appIconTexSize int

	windowDrag      int
	windowDragStart [2]float64
	windowDragPos   [2]int
	windowDragSize  [2]int

	// GLFW cannot move undecorated windows on Wayland, so it keeps the native title bar.
	wayland bool

	win *g.MasterWindow

	relaunchTarget   *DiscordInstall
	relaunchSnapshot *RunningDiscord
	relaunchNote     string
	relaunchErr      error
)

//go:embed winres/icon.png
var iconBytes []byte

func init() {
	LogLevel = LevelDebug
}

type zoomAction int

const (
	zoomNone zoomAction = iota
	zoomIn
	zoomOut
	zoomReset
)

func hasZoomModifier(mod g.Modifier, goos string) bool {
	// Mask out lock keys so shortcuts still work when NumLock or CapsLock is active.
	clean := mod &^ (g.ModNumLock | g.ModCapsLock)
	// Ignore Alt combinations so AltGr (Ctrl+Alt on Windows) never triggers zoom.
	if clean&g.ModAlt != 0 {
		return false
	}
	if clean&g.ModControl != 0 {
		return true
	}
	if goos == "darwin" && clean&g.ModSuper != 0 {
		return true
	}
	return false
}

func classifyZoomKey(key glfwbackend.GLFWKey, keyName string) zoomAction {
	// Numpad keys are layout-independent across keyboard layouts.
	switch key {
	case glfwbackend.GLFWKeyKpAdd:
		return zoomIn
	case glfwbackend.GLFWKeyKpSubtract:
		return zoomOut
	case glfwbackend.GLFWKeyKp0:
		return zoomReset
	}

	// Layout-aware key name from GLFW (handles QWERTZ, AZERTY, Nordic, US QWERTY, etc.).
	switch keyName {
	case "+", "=":
		return zoomIn
	case "-":
		return zoomOut
	case "0":
		return zoomReset
	}

	// Fallback if the platform did not provide a layout key name.
	if keyName == "" {
		switch key {
		case glfwbackend.GLFWKeyEqual:
			return zoomIn
		case glfwbackend.GLFWKeyMinus:
			return zoomOut
		case glfwbackend.GLFWKey0:
			return zoomReset
		}
	}

	return zoomNone
}

func performZoomAction(action zoomAction) bool {
	switch action {
	case zoomIn:
		if setZoomScale(zoomScale + zoomScaleStep) {
			applyZoom()
			return true
		}
	case zoomOut:
		if setZoomScale(zoomScale - zoomScaleStep) {
			applyZoom()
			return true
		}
	case zoomReset:
		if setZoomScale(defaultZoomScale) {
			applyZoom()
			return true
		}
	}
	return false
}

func handleKey(key, scancode, action, modifiers int) {
	if g.Action(action) != g.Press {
		return
	}
	mod := g.Modifier(modifiers) &^ (g.ModNumLock | g.ModCapsLock)
	if hasZoomModifier(mod, runtime.GOOS) {
		name := C.GoString(C.glfwGetKeyName(C.int(key), C.int(scancode)))
		performZoomAction(classifyZoomKey(glfwbackend.GLFWKey(key), name))
	} else if mod == g.ModNone {
		switch glfwbackend.GLFWKey(key) {
		case glfwbackend.GLFWKeyUp:
			if radioIdx > 0 {
				radioIdx--
			}
		case glfwbackend.GLFWKeyDown:
			if radioIdx < maxRadioIdx() {
				radioIdx++
			}
		}
	}
}

func handleWheelZoom() {
	ctrlHeld := g.IsKeyDown(g.KeyLeftControl) || g.IsKeyDown(g.KeyRightControl)
	if runtime.GOOS == "darwin" {
		ctrlHeld = ctrlHeld || g.IsKeyDown(g.KeyLeftSuper) || g.IsKeyDown(g.KeyRightSuper)
	}
	altHeld := g.IsKeyDown(g.KeyLeftAlt) || g.IsKeyDown(g.KeyRightAlt)
	if !ctrlHeld || altHeld {
		return
	}

	wheel := imgui.CurrentIO().MouseWheel()
	if wheel > 0 {
		performZoomAction(zoomIn)
	} else if wheel < 0 {
		performZoomAction(zoomOut)
	}
}

func main() {
	const width, height = 1200, 800
	if runtime.GOOS == "linux" {
		os.Setenv("GDK_SCALE", "1")
		os.Setenv("GDK_DPI_SCALE", "1")
	}

	C.glfwInit()
	wayland = C.glfwGetPlatform() == C.GLFW_PLATFORM_WAYLAND
	win = g.NewMasterWindow("Equilotl", width, height, Ternary(wayland, g.MasterWindowFlags(0), g.MasterWindowFlagsFrameless))
	// GLFW 3.3 scaled the initial window on platforms with pixel-based window coordinates.
	if runtime.GOOS == "windows" || (runtime.GOOS == "linux" && !wayland) {
		scaleX, scaleY := g.Context.Backend().ContentScale()
		if scaleX != 1 || scaleY != 1 {
			win.SetSize(int(snap(width*scaleX)), int(snap(height*scaleY)))
		}
	}
	win.SetSizeLimits(800, 580, -1, -1)
	g.Context.Backend().SetKeyCallback(handleKey)
	applyTheme(win)
	initFonts()

	InitGithubDownloader()
	refreshDiscords()

	go func() {
		<-GithubDoneChan
		githubDone.Store(true)
		g.Update()
	}()

	go func() {
		<-SelfUpdateCheckDoneChan
		g.Update()
	}()

	icon, _, err := image.Decode(bytes.NewReader(iconBytes))
	if err != nil {
		Log.Warn("Failed to load application icon", err)
		Log.Debug(iconBytes, len(iconBytes))
	} else {
		appIcon = icon
		if !wayland {
			win.SetIcon(icon)
		}
	}

	win.Run(loop)
}

func retryGithubFetch() {
	if !githubDone.CompareAndSwap(true, false) {
		return
	}

	FetchGithubRelease()
	doneCh := GithubDoneChan
	g.Update()

	go func() {
		<-doneCh
		githubDone.Store(true)
		g.Update()
	}()
}

func refreshDiscords() {
	prev := getSelectedDiscord()
	wasCustom := showCustomLocation && radioIdx == customChoiceIdx

	discords = FindDiscords()
	customChoiceIdx = len(discords)

	if wasCustom {
		radioIdx = customChoiceIdx
	} else if _, idx := findMatchingInstall(discords, prev); idx != -1 {
		radioIdx = idx
	}
	clampRadioIdx()
}

func maxRadioIdx() int {
	if showCustomLocation {
		return customChoiceIdx
	}
	if len(discords) > 0 {
		return len(discords) - 1
	}
	return 0
}

func clampRadioIdx() {
	maxIdx := maxRadioIdx()
	if radioIdx < 0 {
		radioIdx = 0
	} else if radioIdx > maxIdx {
		radioIdx = maxIdx
	}
}

func hasValidSelection() bool {
	if len(discords) > 0 && radioIdx >= 0 && radioIdx < len(discords) {
		return true
	}
	return showCustomLocation && strings.TrimSpace(customDir) != ""
}

func getSelectedDiscord() *DiscordInstall {
	if radioIdx >= 0 && radioIdx < len(discords) {
		return discords[radioIdx].(*DiscordInstall)
	}
	return nil
}

type CondWidget struct {
	predicate  bool
	ifWidget   func() g.Widget
	elseWidget func() g.Widget
}

func (w *CondWidget) Build() {
	if w.predicate {
		if w.ifWidget != nil {
			w.ifWidget().Build()
		}
	} else if w.elseWidget != nil {
		w.elseWidget().Build()
	}
}

func getChosenInstall() *DiscordInstall {
	if prev := getSelectedDiscord(); prev != nil {
		refreshDiscords()
		if refreshed, idx := findMatchingInstall(discords, prev); refreshed != nil {
			radioIdx = idx
			return refreshed
		}
		return nil
	}

	if !showCustomLocation {
		return nil
	}

	if discord := ParseDiscord(customDir, ""); discord != nil {
		return discord
	}

	if discord := ParseDiscordNew(customDir, "", strings.Contains(customDir, "com.discordapp")); discord != nil {
		return discord
	}

	g.OpenPopup("#invalid-custom-location")
	return nil
}

func InstallLatestBuilds() (err error) {
	if IsDevInstall {
		return
	}

	err = installLatestBuilds()
	if err != nil {
		ShowErrorModal("Failed to install the latest Equicord builds from GitHub", err.Error(), SupportUrl)
	}
	return
}

func handlePatch() {
	choice := getChosenInstall()
	if choice != nil {
		choice.Patch()
	}
}

func handleUnpatch() {
	choice := getChosenInstall()
	if choice != nil {
		choice.Unpatch()
	}
}

func handleOpenAsar() {
	choice := getChosenInstall()
	if choice == nil {
		return
	}
	if acceptedOpenAsar || choice.IsOpenAsar() {
		handleOpenAsarConfirmed(choice)
		return
	}

	g.OpenPopup("#openasar-confirm")
}

func preparePostPatchRelaunch(di *DiscordInstall) {
	relaunchTarget = nil
	relaunchSnapshot = nil
	relaunchNote = ""

	if di == nil {
		return
	}

	name := branchDisplayName(di.branch)
	if runtime.GOOS == "windows" {
		relaunched, err := di.RelaunchKilled()
		if err != nil {
			Log.Warn("Failed to relaunch", name+":", err)
			relaunchNote = "Failed to automatically relaunch " + name + ": " + err.Error() + "\nPlease start it manually."
		} else if relaunched {
			relaunchNote = name + " has been automatically relaunched."
		}
		return
	}

	if running := di.FindRunning(); running != nil {
		relaunchTarget = di
		relaunchSnapshot = running
	}
}

func handleOpenAsarConfirmed(choice *DiscordInstall) {
	if choice == nil {
		return
	}
	if choice.IsOpenAsar() {
		if err := choice.UninstallOpenAsar(); err != nil {
			choice.ClearPendingRelaunch()
			handleErr(choice, err, "uninstall OpenAsar from")
		} else {
			preparePostPatchRelaunch(choice)
			g.OpenPopup("#openasar-unpatched")
			g.Update()
		}
	} else {
		if err := choice.InstallOpenAsar(); err != nil {
			choice.ClearPendingRelaunch()
			handleErr(choice, err, "install OpenAsar on")
		} else {
			preparePostPatchRelaunch(choice)
			g.OpenPopup("#openasar-patched")
			g.Update()
		}
	}
}

func handleErr(di *DiscordInstall, err error, action string) {
	if errors.Is(err, ErrAlreadyReported) {
		return
	}
	if errors.Is(err, os.ErrPermission) {
		switch runtime.GOOS {
		case "windows":
			err = errors.New("Permission denied. Make sure your Discord is fully closed (from the tray)!")
		case "darwin":
			HandleInsufficientPermissions()
			return
		case "linux":
			command := "sudo chown -R \"$USER:$USER\" " + di.path
			err = errors.New("Permission denied. Try to run the installer with sudo privileges.\n\nIf that also doesn't work, try running the following command in your terminal:\n" + command)
		default:
			err = errors.New("Permission denied. Maybe try running me as Administrator/Root?")
		}
	}

	ShowErrorModal("Failed to "+action+" this Install.", err.Error(), SupportUrl)
}

func HandleScuffedInstall() {
	g.OpenPopup("#scuffed-install")
}

func HandleInsufficientPermissions() {
	g.OpenPopup("#insufficient-permissions")
}

func (di *DiscordInstall) Patch() {
	if CheckScuffedInstall() {
		return
	}
	if err := di.patch(); err != nil {
		di.ClearPendingRelaunch()
		handleErr(di, err, "patch")
	} else {
		preparePostPatchRelaunch(di)
		g.OpenPopup("#patched")
	}
}

func (di *DiscordInstall) Unpatch() {
	if err := di.unpatch(); err != nil {
		di.ClearPendingRelaunch()
		handleErr(di, err, "unpatch")
	} else {
		preparePostPatchRelaunch(di)
		g.OpenPopup("#unpatched")
	}
}

func onCustomInputChanged() {
	p := customDir
	if len(p) != 0 {
		// Select the custom option for people
		radioIdx = customChoiceIdx
	}

	if p == "" {
		autoCompleteDir = ""
		autoCompleteFile = ""
		autoCompleteCandidates = nil
		autoCompleteIdx = 0
		lastAutoComplete = ""
		didAutoComplete = false
		return
	}

	dir := path.Dir(p)

	isNewDir := strings.HasSuffix(p, "/") || strings.HasSuffix(p, string(os.PathSeparator))
	wentUpADir := !isNewDir && dir != autoCompleteDir

	if isNewDir || wentUpADir {
		autoCompleteDir = dir
		// reset all the funnies
		autoCompleteIdx = 0
		lastAutoComplete = ""
		autoCompleteFile = ""
		autoCompleteCandidates = nil

		// Generate autocomplete items
		files, err := os.ReadDir(dir)
		if err == nil {
			for _, file := range files {
				autoCompleteCandidates = append(autoCompleteCandidates, file.Name())
			}
		}
	} else if !didAutoComplete {
		// reset auto complete and update our file
		autoCompleteFile = path.Base(p)
		lastAutoComplete = ""
	}

	if wentUpADir {
		autoCompleteFile = path.Base(p)
	}

	didAutoComplete = false
}

// go can you give me []any?
// to pass to giu RangeBuilder?
// yeeeeees
// actually returns []string like a boss
func makeAutoComplete() []any {
	input := strings.ToLower(autoCompleteFile)

	var candidates []any
	for _, e := range autoCompleteCandidates {
		file := strings.ToLower(e)
		if autoCompleteFile == "" || strings.HasPrefix(file, input) {
			candidates = append(candidates, e)
		}
	}
	return candidates
}

func makeRadioOnChange(i int) func() {
	return func() {
		radioIdx = i
	}
}

func equicordCommitUrl(hash string) string {
	return EquicordRepoUrl + "/commit/" + hash
}

func installerCommitUrl(hash string) string {
	return InstallerRepoUrl + "/commit/" + hash
}

func Tooltip(label string) g.Widget {
	return fontRegular(18).style().
		SetColor(g.StyleColorPopupBg, ThemeBg0).
		SetColor(g.StyleColorBorder, ThemeFg0Muted).
		SetColor(g.StyleColorText, ThemeFg1).
		SetStyle(g.StyleVarWindowPadding, scaled(10), scaled(8)).
		SetStyleFloat(g.StyleVarWindowRounding, scaled(8)).
		SetStyleFloat(g.StyleVarPopupBorderSize, 1).
		To(
			g.Tooltip(label),
		)
}

func styledModal(id string, content ...g.Widget) g.Widget {
	return fontRegular(20).style().
		SetColor(g.StyleColorPopupBg, ThemeBgCurrentWord).
		SetColor(g.StyleColorBorder, ThemeFg0Muted).
		SetStyle(g.StyleVarWindowPadding, scaled(28), scaled(24)).
		SetStyleFloat(g.StyleVarWindowRounding, scaled(14)).
		SetStyleFloat(g.StyleVarPopupBorderSize, 1.5).
		To(
			g.PopupModal(id).
				Flags(g.WindowFlagsNoTitleBar | g.WindowFlagsAlwaysAutoResize).
				Layout(
					g.Custom(func() {
						imgui.PushTextWrapPosV(scaled(620))
						g.Layout(content).Build()
						imgui.PopTextWrapPos()
					}),
				),
		)
}

func InfoModal(id, title, description string) g.Widget {
	return RawInfoModal(id, title, description, "", false)
}

func InfoModalExtra(id, title, description, extra string) g.Widget {
	return RawInfoModal(id, title, description, extra, false)
}

func ErrorModal(id, title, errText, supportLink string) g.Widget {
	return styledModal(
		id,
		fontSemiBold(30).style().
			SetColor(g.StyleColorText, ThemeAccentRed).
			To(g.Label(title)),
		g.Dummy(0, scaled(6)),
		g.Custom(func() {
			linkFont := fontSemiBold(20)
			labelFont := fontRegular(20)
			padX := scaled(3)
			padY := scaled(2)
			var rowH float32
			labelFont.style().To(g.Custom(func() {
				_, textH := g.CalcTextSize("Ag")
				rowH = textH + padY*2
			})).Build()
			renderAlignedRow(
				rowH,
				scaled(2),
				rowAlignedLabel("If this issue persists, visit", ThemeFg0, labelFont),
				rowAlignedLink("our support page", supportLink, linkFont, padX, padY, func() {
					g.OpenURL(supportLink)
				}),
			).Build()
		}),
		g.Dummy(0, scaled(14)),
		fontSemiBold(18).style().
			SetColor(g.StyleColorText, ThemeGrey2).
			To(g.Label("Error Information")),
		g.Dummy(0, scaled(4)),
		g.Style().
			SetColor(g.StyleColorChildBg, ThemeBg0).
			SetColor(g.StyleColorBorder, ThemeFg0Muted2).
			SetStyle(g.StyleVarWindowPadding, scaled(14), scaled(12)).
			SetStyleFloat(g.StyleVarChildRounding, scaled(10)).
			SetStyleFloat(g.StyleVarChildBorderSize, 1).
			To(
				g.Child().
					Border(true).
					Size(scaled(580), scaled(130)).
					Layout(
						fontRegular(18).style().
							SetColor(g.StyleColorText, ThemeFg1).
							To(g.Label(errText).Wrapped(true)),
					),
			),
		g.Dummy(0, scaled(16)),
		fontSemiBold(20).style().To(
			accentButton("Ok", ThemeAccentPurple, scaled(120), scaled(40), false, "", func() {
				g.CloseCurrentPopup()
			}),
		),
	)
}

func RawInfoModal(id, title, description, extra string, isOpenAsar bool) g.Widget {
	titleColor := ThemeAccentPurple
	switch id {
	case "#patched", "#openasar-patched":
		titleColor = ThemeAccentGreen
	case "#scuffed-install", "#insufficient-permissions", "#invalid-custom-location":
		titleColor = ThemeAccentYellow
	}

	return styledModal(
		id,
		fontSemiBold(30).style().
			SetColor(g.StyleColorText, titleColor).
			To(g.Label(title)),
		g.Dummy(0, scaled(8)),
		fontRegular(20).style().
			SetColor(g.StyleColorText, ThemeFg0).
			To(g.Label(description)),
		&CondWidget{extra != "", func() g.Widget {
			return g.Column(
				g.Dummy(0, scaled(10)),
				g.Style().
					SetColor(g.StyleColorChildBg, ThemeBg0).
					SetColor(g.StyleColorBorder, ThemeFg0Muted2).
					SetStyle(g.StyleVarWindowPadding, scaled(12), scaled(10)).
					SetStyleFloat(g.StyleVarChildRounding, scaled(10)).
					SetStyleFloat(g.StyleVarChildBorderSize, 1).
					To(
						g.Child().
							Border(true).
							Size(scaled(580), scaled(110)).
							Layout(
								fontRegular(18).style().
									SetColor(g.StyleColorText, ThemeFg1).
									To(g.Label(extra).Wrapped(true)),
							),
					),
			)
		}, nil},
		&CondWidget{id == "#scuffed-install", func() g.Widget {
			return g.Column(
				g.Dummy(0, scaled(12)),
				fontSemiBold(20).style().To(
					accentButton("Take me there!", ThemeAccentBlue, scaled(210), scaled(40), false, "", func() {
						// this issue only exists on windows so using Windows specific path is oki
						username := os.Getenv("USERNAME")
						programData := os.Getenv("PROGRAMDATA")
						g.OpenURL("file://" + path.Join(programData, username))
					}),
				),
			)
		}, nil},
		&CondWidget{id == "#insufficient-permissions", func() g.Widget {
			return g.Column(
				g.Dummy(0, scaled(12)),
				fontSemiBold(20).style().To(
					accentButton("Open Settings", ThemeAccentBlue, scaled(210), scaled(40), false, "", func() {
						// "App Management" permissions doesnt exist on Monterey, just use full disk access for now
						g.OpenURL("x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles")
					}),
				),
			)
		}, nil},
		g.Dummy(0, scaled(18)),
		&CondWidget{isOpenAsar,
			func() g.Widget {
				return fontSemiBold(20).style().To(
					g.Row(
						accentButton("Accept", ThemeAccentPurple, scaled(125), scaled(40), false, "", func() {
							acceptedOpenAsar = true
							g.CloseCurrentPopup()
						}),
						secondaryButton("Cancel", scaled(125), scaled(40), false, "", func() {
							g.CloseCurrentPopup()
						}),
					),
				)
			},
			func() g.Widget {
				return fontSemiBold(20).style().To(
					accentButton("Ok", ThemeAccentPurple, scaled(120), scaled(40), false, "", func() {
						g.CloseCurrentPopup()
					}),
				)
			},
		},
	)
}

func PostActionModal(id, title, shortDesc, fallbackDesc string) g.Widget {
	titleColor := ThemeAccentPurple
	if id == "#patched" || id == "#openasar-patched" {
		titleColor = ThemeAccentGreen
	}

	return styledModal(
		id,
		g.Custom(func() {
			description := fallbackDesc
			if relaunchTarget != nil {
				description = shortDesc + "\n\n" + branchDisplayName(relaunchTarget.branch) + " is currently running. Would you like to relaunch it now?"
			} else if relaunchNote != "" {
				description = shortDesc + "\n\n" + relaunchNote
			}

			var buttons g.Widget
			if relaunchTarget != nil {
				target := relaunchTarget
				snapshot := relaunchSnapshot
				buttons = fontSemiBold(20).style().To(
					g.Row(
						accentButton("Relaunch Now", ThemeAccentPurple, scaled(175), scaled(40), false, "", func() {
							relaunchTarget = nil
							relaunchSnapshot = nil
							if err := target.Relaunch(snapshot); err != nil {
								relaunchErr = err
							}
							g.CloseCurrentPopup()
						}),
						secondaryButton("Not Now", scaled(125), scaled(40), false, "", func() {
							relaunchTarget = nil
							relaunchSnapshot = nil
							g.CloseCurrentPopup()
						}),
					),
				)
			} else {
				buttons = fontSemiBold(20).style().To(
					accentButton("Ok", ThemeAccentPurple, scaled(120), scaled(40), false, "", func() {
						g.CloseCurrentPopup()
					}),
				)
			}

			g.Layout{
				fontSemiBold(30).style().
					SetColor(g.StyleColorText, titleColor).
					To(g.Label(title)),
				g.Dummy(0, scaled(8)),
				fontRegular(20).style().
					SetColor(g.StyleColorText, ThemeFg0).
					To(g.Label(description)),
				g.Dummy(0, scaled(18)),
				buttons,
			}.Build()
		}),
	)
}

func UpdateModal() g.Widget {
	return styledModal(
		"#update-prompt",
		fontSemiBold(30).style().
			SetColor(g.StyleColorText, ThemeAccentPurple).
			To(g.Label("Your Installer is outdated!")),
		g.Dummy(0, scaled(8)),
		fontRegular(20).style().
			SetColor(g.StyleColorText, ThemeFg0).
			To(
				g.Label(
					"Would you like to update now?\n\n"+
						"Once you press Update Now, the new installer will automatically be downloaded.\n"+
						"The installer will temporarily seem unresponsive. Just wait!\n"+
						"Once the update is done, the Installer will automatically reopen.\n\n"+
						"On macOS, auto-updates are not supported, so the download will open in your browser.",
				),
			),
		g.Dummy(0, scaled(18)),
		fontSemiBold(20).style().To(
			g.Row(
				accentButton("Update Now", ThemeAccentPurple, scaled(160), scaled(40), false, "", func() {
					if runtime.GOOS == "darwin" {
						g.CloseCurrentPopup()
						g.OpenURL(GetInstallerDownloadLink())
						return
					}

					err := UpdateSelf()
					g.CloseCurrentPopup()

					if err != nil {
						ShowModal("Failed to update self!", "Please manually download the latest Installer.", err.Error())
					} else {
						if err = RelaunchSelf(); err != nil {
							ShowModal("Failed to restart self!", "Please manually restart the Installer.", err.Error())
						}
					}
				}),
				secondaryButton("Later", scaled(125), scaled(40), false, "", func() {
					g.CloseCurrentPopup()
				}),
			),
		),
	)
}

func ShowModal(title, desc, extra string) {
	modalTitle = title
	modalMessage = desc
	modalExtra = extra
	modalId++
	g.OpenPopup("#modal" + strconv.Itoa(modalId))
}

func ShowErrorModal(title, extra, supportLink string) {
	modalTitle = title
	modalExtra = extra
	modalSupportLink = supportLink
	modalId++
	g.OpenPopup("#error-modal" + strconv.Itoa(modalId))
}

func renderHeaderMetadata() g.Widget {
	metaFont := fontRegular(18)
	metaSemi := fontSemiBold(18)
	padX := scaled(4)
	padY := scaled(2)

	var items []alignedRowItem
	hasInstallerMeta := false

	hasTag := buildinfo.InstallerTag != "" && buildinfo.InstallerTag != buildinfo.VersionUnknown
	hasHash := buildinfo.InstallerGitHash != "" && buildinfo.InstallerGitHash != buildinfo.VersionUnknown

	if hasTag {
		items = append(items, rowAlignedLabel("Installer "+buildinfo.InstallerTag, ThemeGrey2, metaFont))
		hasInstallerMeta = true
	}
	if hasHash {
		if !hasTag {
			items = append(items, rowAlignedLabel("Installer", ThemeGrey2, metaFont))
		}
		hash := buildinfo.InstallerGitHash
		url := installerCommitUrl(hash)
		items = append(items, rowAlignedLink(ShortHash(hash), "Open Installer commit "+ShortHash(hash)+" on GitHub", metaSemi, padX, padY, func() {
			g.OpenURL(url)
		}))
		hasInstallerMeta = true
	}

	var buildItems []alignedRowItem
	switch {
	case IsDevInstall:
		buildItems = append(buildItems, rowAlignedLabel("Dev Install", ThemeAccentYellow, metaSemi))
	case GithubError != nil:
		buildItems = append(buildItems, rowAlignedLabel("Offline / Release fetch failed", ThemeAccentRed, metaSemi))
	case !githubDone.Load():
		buildItems = append(buildItems, rowAlignedLabel("Checking latest Equicord build...", ThemeGrey1, metaFont))
	case LatestHash != "" && LatestHash != "Unknown":
		hash := LatestHash
		url := equicordCommitUrl(hash)
		buildItems = append(buildItems,
			rowAlignedLabel("Latest Equicord:", ThemeGrey2, metaFont),
			rowAlignedLink(ShortHash(hash), "Open Equicord commit "+ShortHash(hash)+" on GitHub", metaSemi, padX, padY, func() {
				g.OpenURL(url)
			}),
		)
		if InstalledHash == LatestHash {
			buildItems = append(buildItems, rowAlignedLabel("(Up to date)", ThemeAccentGreen, metaFont))
		} else if InstalledHash != "" && InstalledHash != "None" {
			installed := InstalledHash
			installedUrl := equicordCommitUrl(installed)
			buildItems = append(buildItems,
				rowAlignedLabel("|", ThemeGrey0, metaFont),
				rowAlignedLabel("Installed:", ThemeGrey2, metaFont),
				rowAlignedLink(ShortHash(installed), "Open installed Equicord commit "+ShortHash(installed)+" on GitHub", metaSemi, padX, padY, func() {
					g.OpenURL(installedUrl)
				}),
			)
		}
	}

	if hasInstallerMeta && len(buildItems) > 0 {
		items = append(items, rowAlignedLabel("|", ThemeGrey0, metaFont))
	}
	items = append(items, buildItems...)

	if len(items) == 0 {
		return g.Dummy(0, 0)
	}

	return g.Custom(func() {
		var rowH float32
		metaFont.style().To(g.Custom(func() {
			_, textH := g.CalcTextSize("Ag")
			rowH = textH + padY*2
		})).Build()
		renderAlignedRow(rowH, scaled(6), items...).Build()
	})
}

func renderHeader() g.Widget {
	return g.Custom(func() {
		iconSize := scaled(60)
		startPos := imgui.CursorPos()
		startX := snap(startPos.X)
		startY := snap(startPos.Y)

		if appIcon != nil {
			var titleH, metaH float32
			fontSemiBold(40).style().To(g.Custom(func() {
				_, titleH = g.CalcTextSize("Equilotl")
			})).Build()
			fontRegular(18).style().To(g.Custom(func() {
				_, metaH = g.CalcTextSize("Ag")
			})).Build()

			stackH := titleH + scaled(8) + metaH + scaled(4)
			iconY := startY
			if stackH > iconSize {
				iconY = snap(startY + (stackH-iconSize)/2)
			}
			imgui.SetCursorPos(imgui.Vec2{X: startX, Y: iconY})
			if px := int(iconSize * imgui.CurrentIO().DisplayFramebufferScale().X); px != appIconTexSize {
				appIconTexSize = px
				scaledIcon := image.NewRGBA(image.Rect(0, 0, px, px))
				draw.CatmullRom.Scale(scaledIcon, scaledIcon.Bounds(), appIcon, appIcon.Bounds(), draw.Src, nil)
				g.NewTextureFromRgba(scaledIcon, func(t *g.Texture) {
					appIconTex = t
					g.Update()
				})
			}
			g.Image(appIconTex).Size(iconSize, iconSize).Build()

			imgui.SetCursorPos(imgui.Vec2{X: startX + iconSize + scaled(16), Y: startY})
		}

		g.Column(
			fontSemiBold(40).style().
				SetColor(g.StyleColorText, ThemeFg1).
				To(g.Label("Equilotl")),
			renderHeaderMetadata(),
		).Build()
	})
}

func renderInstallListSection(candidates []any, maxListHeight float32) g.Widget {
	headingFont := fontSemiBold(28)
	radioFont := fontSemiBold(20)
	badgeFont := fontSemiBold(18)
	pathFont := fontRegular(20)

	return renderAutoHeightCard(
		"InstallSection",
		color.RGBA{},
		color.RGBA{},
		0,
		0,
		0,
		0,
		scaled(180),
		maxListHeight,
		g.Custom(func() {
			startPos := imgui.CursorPos()
			startX := snap(startPos.X)
			startY := snap(startPos.Y)
			availW, _ := g.GetAvailableRegion()
			refreshW := scaled(115)
			refreshH := scaled(34)

			var titleH float32
			headingFont.style().
				SetColor(g.StyleColorText, ThemeFg1).
				To(g.Custom(func() {
					_, titleH = g.CalcTextSize("Select a Discord Install")
					rowH := titleH
					if refreshH > rowH {
						rowH = refreshH
					}
					imgui.SetCursorPos(imgui.Vec2{X: startX, Y: snap(startY + (rowH-titleH)/2)})
					g.Label("Select a Discord Install").Build()
				})).Build()

			rowH := titleH
			if refreshH > rowH {
				rowH = refreshH
			}
			targetX := snap(startX + availW - refreshW)
			imgui.SetCursorPos(imgui.Vec2{X: targetX, Y: snap(startY + (rowH-refreshH)/2)})
			fontSemiBold(18).style().To(
				secondaryButton("Refresh", refreshW, refreshH, false, "Re-scan for Discord installations", refreshDiscords),
			).Build()

			imgui.SetCursorPos(imgui.Vec2{X: startX, Y: snap(startY + rowH)})
			imgui.Dummy(imgui.Vec2{X: 0, Y: 0})
		}),
		g.Dummy(0, scaled(2)),
		g.Separator(),
		g.Dummy(0, scaled(6)),

		&CondWidget{len(discords) == 0, func() g.Widget {
			s := "No Discord installs found. You first need to install Discord."
			if runtime.GOOS == "linux" {
				s += " Snap is not supported."
			}

			return &CondWidget{!showCustomLocation, func() g.Widget {
				return g.Column(
					fontRegular(24).style().
						SetColor(g.StyleColorText, ThemeFg0).
						To(g.Label(s).Wrapped(true)),
					g.Dummy(0, scaled(10)),
					fontRegular(18).style().To(
						g.Checkbox("I am an advanced user and have Discord installed at a different location", &showCustomLocation).
							OnChange(func() {
								if showCustomLocation {
									radioIdx = customChoiceIdx
								} else {
									clampRadioIdx()
								}
							}),
					),
				)
			}, nil}
		}, nil},

		g.RangeBuilder("Discords", discords, func(i int, v any) g.Widget {
			d := v.(*DiscordInstall)
			label := branchDisplayName(d.branch)

			items := []alignedRowItem{
				rowAlignedRadio(label+"##discord_"+strconv.Itoa(i), radioIdx == i, radioFont, makeRadioOnChange(i)),
			}

			if d.isPatched {
				items = append(items, rowAlignedLabel("[Equicord Installed]", ThemeAccentGreen, badgeFont))
			}
			if d.isOpenAsar != nil && *d.isOpenAsar {
				items = append(items, rowAlignedLabel("[OpenAsar]", ThemeAccentAqua, badgeFont))
			}
			if d.isFlatpak {
				items = append(items, rowAlignedLabel("[Flatpak]", ThemeAccentBlue, badgeFont))
			}

			items = append(items, rowAlignedLabel(d.path, ThemeGrey1, pathFont))

			return g.Custom(func() {
				var rowH float32
				radioFont.style().To(g.Custom(func() {
					_, framePadY := g.GetFramePadding()
					_, textH := g.CalcTextSize("Ag")
					rowH = textH + framePadY*2
				})).Build()
				renderAlignedRow(rowH, scaled(10), items...).Build()
			})
		}),

		&CondWidget{showCustomLocation, func() g.Widget {
			return radioFont.style().To(
				g.RadioButton("Custom Install Location", radioIdx == customChoiceIdx).
					OnChange(makeRadioOnChange(customChoiceIdx)),
			)
		}, nil},

		&CondWidget{showCustomLocation, func() g.Widget {
			return g.Custom(func() {
				availW, _ := g.GetAvailableRegion()
				g.Column(
					g.Dummy(0, scaled(6)),
					fontRegular(20).style().
						SetColor(g.StyleColorFrameBg, ThemeBg0).
						SetColor(g.StyleColorFrameBgHovered, ThemeBg0).
						SetColor(g.StyleColorFrameBgActive, ThemeBg0).
						SetColor(g.StyleColorBorder, ThemeFg0Muted).
						SetStyle(g.StyleVarFramePadding, scaled(14), scaled(12)).
						SetStyleFloat(g.StyleVarFrameBorderSize, 1).
						SetStyleFloat(g.StyleVarFrameRounding, scaled(10)).
						To(
							g.InputText(&customDir).Hint("Enter path to your Discord install folder (Tab to autocomplete)").
								Size(availW).
								Flags(g.InputTextFlagsCallbackCompletion).
								OnChange(onCustomInputChanged).
								// this library has its own autocomplete but it's broken
								Callback(
									func(data imgui.InputTextCallbackData) int {
										if data.EventFlag() != imgui.InputTextFlagsCallbackCompletion || len(candidates) == 0 {
											return 0
										}
										// just wrap around
										if autoCompleteIdx >= len(candidates) {
											autoCompleteIdx = 0
										}

										// used by change handler
										didAutoComplete = true

										start := len(customDir)
										// Delete previous auto complete
										if lastAutoComplete != "" {
											start -= len(lastAutoComplete)
											data.DeleteChars(int32(start), int32(len(lastAutoComplete)))
										} else if autoCompleteFile != "" { // delete partial input
											start -= len(autoCompleteFile)
											data.DeleteChars(int32(start), int32(len(autoCompleteFile)))
										}

										// Insert auto complete
										lastAutoComplete = candidates[autoCompleteIdx].(string)
										data.InsertChars(int32(start), lastAutoComplete)
										autoCompleteIdx++

										return 0
									},
								),
						),
					&CondWidget{len(candidates) > 0, func() g.Widget {
						return g.Column(
							g.Dummy(0, scaled(6)),
							fontRegular(18).style().
								SetColor(g.StyleColorChildBg, ThemeBg0).
								SetColor(g.StyleColorBorder, ThemeFg0Muted2).
								SetStyle(g.StyleVarWindowPadding, scaled(10), scaled(8)).
								SetStyleFloat(g.StyleVarChildRounding, scaled(8)).
								SetStyleFloat(g.StyleVarChildBorderSize, 1).
								To(
									g.Child().
										ID("AutoCompleteList").
										Border(true).
										Size(g.Auto, scaled(120)).
										Layout(
											g.RangeBuilder("AutoComplete", candidates, func(i int, v any) g.Widget {
												entry := v.(string)
												return g.Selectable(entry).OnClick(func() {
													customDir = path.Join(autoCompleteDir, entry)
													onCustomInputChanged()
												})
											}),
										),
								),
						)
					}, nil},
				).Build()
			})
		}, nil},
	)
}

func renderActionButtons(currentDiscord *DiscordInstall, isOpenAsar bool) g.Widget {
	return g.Custom(func() {
		availW, _ := g.GetAvailableRegion()
		spacingX, _ := g.GetItemSpacing()
		btnW := snap((availW - spacingX*3) / 4)
		if btnW < scaled(130) {
			btnW = scaled(130)
		}
		btnH := scaled(52)

		noSelection := !hasValidSelection()
		waitingGithub := !IsDevInstall && !githubDone.Load()
		installDisabled := GithubError != nil || waitingGithub || noSelection
		actionDisabled := noSelection

		installTip := "Patch the selected Discord install with Equicord"
		if GithubError != nil {
			installTip = "Unavailable: failed to fetch Equicord release info from GitHub"
		} else if waitingGithub {
			installTip = "Waiting for latest Equicord release info from GitHub..."
		} else if noSelection {
			installTip = "Select a Discord installation first"
		}

		repairTip := "Re-download the latest Equicord build and repair the selected install"
		if installDisabled {
			repairTip = installTip
		}

		openAsarLabel := Ternary(isOpenAsar, "Uninstall OpenAsar", Ternary(currentDiscord != nil, "Install OpenAsar", "(Un-)Install OpenAsar"))
		openAsarTip := Ternary(isOpenAsar, "Restore Discord's original app.asar", "Install OpenAsar for faster startup & performance")

		fontSemiBold(20).style().To(
			g.Row(
				accentButton("Install", ThemeFg1, btnW, btnH, installDisabled, installTip, handlePatch),
				secondaryButton("Reinstall / Repair", btnW, btnH, installDisabled, repairTip, func() {
					if IsDevInstall {
						handlePatch()
					} else {
						err := InstallLatestBuilds()
						if err == nil {
							handlePatch()
						}
					}
				}),
				secondaryButton("Uninstall", btnW, btnH, actionDisabled, "Remove Equicord from the selected Discord install", handleUnpatch),
				secondaryButton(openAsarLabel, btnW, btnH, actionDisabled, openAsarTip, handleOpenAsar),
			),
		).Build()
	})
}

func renderFooter() g.Widget {
	footerFont := fontRegular(18)
	padX := scaled(6)
	padY := scaled(3)

	items := []alignedRowItem{
		rowAlignedLink("equicord.org", "https://equicord.org", footerFont, padX, padY, func() {
			g.OpenURL("https://equicord.org")
		}),
		rowAlignedLabel("|", ThemeGrey0, footerFont),
		rowAlignedLink("Support", SupportUrl, footerFont, padX, padY, func() {
			g.OpenURL(SupportUrl)
		}),
		rowAlignedLabel("|", ThemeGrey0, footerFont),
		rowAlignedLink("GitHub", EquicordRepoUrl, footerFont, padX, padY, func() {
			g.OpenURL(EquicordRepoUrl)
		}),
	}

	if zoomScale != defaultZoomScale {
		pct := int(math.Round(float64(zoomScale) * 100))
		items = append(items,
			rowAlignedLabel("|", ThemeGrey0, footerFont),
			rowAlignedLink(fmt.Sprintf("Zoom %d%% (Reset)", pct), "Click or press Ctrl+0 to reset zoom", footerFont, padX, padY, func() {
				performZoomAction(zoomReset)
			}),
		)
	}

	return g.Custom(func() {
		var rowH float32
		footerFont.style().To(g.Custom(func() {
			_, textH := g.CalcTextSize("Ag")
			rowH = textH + padY*2
		})).Build()
		renderAlignedRow(rowH, scaled(6), items...).Build()
	})
}

func renderInstaller() g.Widget {
	candidates := makeAutoComplete()
	clampRadioIdx()

	currentDiscord := getSelectedDiscord()
	isOpenAsar := currentDiscord != nil && currentDiscord.IsOpenAsar()

	if CanUpdateSelf() && !showedUpdatePrompt {
		showedUpdatePrompt = true
		g.OpenPopup("#update-prompt")
	}

	if relaunchErr != nil {
		err := relaunchErr
		relaunchErr = nil
		ShowModal("Failed to relaunch Discord", "Please restart Discord manually.", err.Error())
	}

	return g.Layout{
		renderCalloutCard(
			"OfficialDownloadNoticeCard",
			ThemeAccentYellow,
			ThemeBgCurrentWord,
			"Official Download Notice",
			"GitHub and equicord.org are the only official places to get Equicord. Any other site claiming to be us is malicious.\n"+
				"If you downloaded from any other source, delete / uninstall everything immediately, run a malware scan, and change your Discord password.",
			nil,
		),

		g.Dummy(0, scaled(16)),

		// Shrink tightly to fit installs, capping at remaining space above the buttons & footer so they never clip.
		g.Custom(func() {
			_, availH := g.GetAvailableRegion()
			reservedBottom := scaled(124) // gap + 52px buttons + spacing + footer
			maxListH := availH - reservedBottom
			if maxListH < scaled(160) {
				maxListH = scaled(160)
			}
			renderInstallListSection(candidates, maxListH).Build()
		}),

		g.Dummy(0, scaled(18)),

		renderActionButtons(currentDiscord, isOpenAsar),

		g.Dummy(0, scaled(10)),

		renderFooter(),

		PostActionModal("#patched", "Installed!", "Equicord was successfully installed!", "Equicord was successfully installed!"),
		PostActionModal("#unpatched", "Uninstalled", "Equicord has been uninstalled!", "Equicord has been uninstalled!"),
		InfoModal("#scuffed-install", "Hold On!", "You have a broken Discord Install.\n"+
			"Sometimes Discord decides to install to the wrong location for some reason!\n"+
			"You need to fix this before patching, otherwise Equicord will likely not work.\n\n"+
			"Use the below button to jump there and delete any folder called Discord or Squirrel.\n"+
			"If the folder is now empty, feel free to go back a step and delete that folder too.\n"+
			"Then see if Discord still starts. If not, reinstall it."),
		RawInfoModal("#openasar-confirm", "OpenAsar", "OpenAsar is an open-source alternative of Discord desktop's app.asar.\n"+
			"Equicord is in no way affiliated with OpenAsar.\n"+
			"You're installing OpenAsar at your own risk. If you run into issues with OpenAsar,\n"+
			"no support will be provided, join the OpenAsar Server instead!\n\n"+
			"To install OpenAsar, press Accept and click 'Install OpenAsar' again.", "", true),
		InfoModal("#insufficient-permissions", "Insufficient Permissions", "Permission denied. Please grant the installer permissions in the settings."),
		PostActionModal("#openasar-patched", "Successfully Installed OpenAsar", "OpenAsar was successfully installed!", "If Discord is still open, fully close it first. Then start it again and verify OpenAsar installed successfully!"),
		PostActionModal("#openasar-unpatched", "Successfully Uninstalled OpenAsar", "OpenAsar has been uninstalled!", "If Discord is still open, fully close it first. Then start it again and it should be back to stock!"),
		InfoModal("#invalid-custom-location", "Invalid Location", "The specified location is not a valid Discord install.\nMake sure you select the base folder.\n\nHint: Discord snap is not supported. Use flatpak or .deb."),
		InfoModalExtra("#modal"+strconv.Itoa(modalId), modalTitle, modalMessage, modalExtra),
		ErrorModal("#error-modal"+strconv.Itoa(modalId), modalTitle, modalExtra, modalSupportLink),
		UpdateModal(),
	}
}

func loop() {
	handleWheelZoom()
	pushedVars := pushGlobalStyleVars()

	g.SingleWindow().
		Layout(
			g.Custom(func() {
				pos, size := imgui.WindowPos(), imgui.WindowSize()
				dl := imgui.WindowDrawList()
				imgui.PushClipRect(pos, imgui.Vec2{X: pos.X + size.X, Y: pos.Y + size.Y}, false)
				defer imgui.PopClipRect()
				const bands = 32
				t := func(p imgui.Vec2) float32 {
					return ((p.X-pos.X)/size.X + (p.Y-pos.Y)/size.Y) / 2
				}
				for k := 0; k < bands; k++ {
					lo, hi := float32(k)/bands, float32(k+1)/bands
					poly := []imgui.Vec2{pos, {X: pos.X + size.X, Y: pos.Y}, {X: pos.X + size.X, Y: pos.Y + size.Y}, {X: pos.X, Y: pos.Y + size.Y}}
					for _, edge := range []float32{lo, -hi} {
						var out []imgui.Vec2
						for i, a := range poly {
							b := poly[(i+1)%len(poly)]
							da, db := t(a)-edge, t(b)-edge
							if edge < 0 {
								da, db = -t(a)-edge, -t(b)-edge
							}
							if da >= 0 {
								out = append(out, a)
							}
							if (da >= 0) != (db >= 0) {
								f := da / (da - db)
								out = append(out, imgui.Vec2{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f})
							}
						}
						poly = out
					}
					if len(poly) < 3 {
						continue
					}
					dl.PathClear()
					for _, p := range poly {
						dl.PathLineTo(p)
					}
					dl.PathFillConvex(g.ColorToUint(blendColor(ThemeBg1, ThemeBgCurrentWord, (lo+hi)/2)))
				}
			}),
			g.Custom(func() {
				if wayland {
					return
				}
				gw := C.glfwGetCurrentContext()
				var cx, cy float64
				C.glfwGetCursorPos(gw, (*C.double)(&cx), (*C.double)(&cy))
				wx, wy := win.GetPos()
				ww, wh := win.GetSize()
				sx, sy := float64(wx)+cx, float64(wy)+cy
				maximized := C.glfwGetWindowAttrib(gw, C.GLFW_MAXIMIZED) != 0
				pos, size := imgui.WindowPos(), imgui.WindowSize()
				start := imgui.CursorPos()
				barH, btnW := scaled(36), scaled(46)
				dl := imgui.WindowDrawList()
				imgui.PushClipRect(pos, imgui.Vec2{X: pos.X + size.X, Y: pos.Y + size.Y}, false)
				defer imgui.PopClipRect()

				if windowDrag != 0 {
					if C.glfwGetMouseButton(gw, 0) == 0 {
						windowDrag = 0
					} else if dx, dy := int(sx-windowDragStart[0]), int(sy-windowDragStart[1]); windowDrag < 0 {
						win.SetPos(windowDragPos[0]+dx, windowDragPos[1]+dy)
					} else {
						x, y, w, h := windowDragPos[0], windowDragPos[1], windowDragSize[0], windowDragSize[1]
						if windowDrag&1 != 0 {
							x, w = x+dx, w-dx
						}
						if windowDrag&2 != 0 {
							w += dx
						}
						if windowDrag&4 != 0 {
							y, h = y+dy, h-dy
						}
						if windowDrag&8 != 0 {
							h += dy
						}
						if w < 800 {
							if windowDrag&1 != 0 {
								x -= 800 - w
							}
							w = 800
						}
						if h < 580 {
							if windowDrag&4 != 0 {
								y -= 580 - h
							}
							h = 580
						}
						win.SetPos(x, y)
						win.SetSize(w, h)
					}
				}

				edge := 0
				if !maximized && cx >= 0 && cy >= 0 && cx < float64(ww) && cy < float64(wh) {
					m := float64(scaled(6))
					if cx < m {
						edge |= 1
					} else if cx > float64(ww)-m {
						edge |= 2
					}
					if cy < m {
						edge |= 4
					} else if cy > float64(wh)-m {
						edge |= 8
					}
				}
				switch cursor := Ternary(windowDrag > 0, windowDrag, edge); {
				case cursor == 5 || cursor == 10:
					imgui.SetMouseCursor(imgui.MouseCursorResizeNWSE)
				case cursor == 6 || cursor == 9:
					imgui.SetMouseCursor(imgui.MouseCursorResizeNESW)
				case cursor&3 != 0:
					imgui.SetMouseCursor(imgui.MouseCursorResizeEW)
				case cursor&12 != 0:
					imgui.SetMouseCursor(imgui.MouseCursorResizeNS)
				}

				macOS := runtime.GOOS == "darwin"
				titlebarStart := pos.X
				titlebarWidth := size.X - 3*btnW
				if macOS {
					titlebarStart += scaled(78)
					titlebarWidth = size.X - scaled(78)
				}
				imgui.SetCursorScreenPos(imgui.Vec2{X: titlebarStart, Y: pos.Y})
				imgui.InvisibleButton("##titlebar", imgui.Vec2{X: titlebarWidth, Y: barH})
				switch {
				case windowDrag != 0 || !imgui.IsMouseClickedBool(imgui.MouseButtonLeft):
				case edge != 0:
					windowDrag = edge
				case imgui.IsItemHovered() && imgui.IsMouseDoubleClicked(imgui.MouseButtonLeft):
					if maximized {
						C.glfwRestoreWindow(gw)
					} else {
						C.glfwMaximizeWindow(gw)
					}
				case imgui.IsItemHovered() && !maximized:
					windowDrag = -1
				}
				if windowDrag != 0 && imgui.IsMouseClickedBool(imgui.MouseButtonLeft) {
					windowDragStart = [2]float64{sx, sy}
					windowDragPos = [2]int{wx, wy}
					windowDragSize = [2]int{ww, wh}
				}

				iconCol := g.ColorToUint(ThemeFg0)
				if macOS {
					colors := []color.RGBA{{255, 95, 87, 255}, {254, 188, 46, 255}, {40, 200, 64, 255}}
					for i, id := range []string{"##close", "##minimize", "##maximize"} {
						cx := pos.X + scaled(float32(18+i*20))
						cy := pos.Y + barH/2
						hitSize := scaled(20)
						imgui.SetCursorScreenPos(imgui.Vec2{X: cx - hitSize/2, Y: cy - hitSize/2})
						clicked := imgui.InvisibleButton(id, imgui.Vec2{X: hitSize, Y: hitSize})
						hovered := imgui.IsItemHovered()
						center := imgui.Vec2{X: snap(cx), Y: snap(cy)}
						radius := scaled(6)
						dl.AddCircleFilled(center, radius, g.ColorToUint(colors[i]))
						dl.AddCircle(center, radius, g.ColorToUint(color.RGBA{0, 0, 0, 45}))
						if hovered {
							mark := scaled(3)
							markCol := g.ColorToUint(color.RGBA{80, 50, 40, 255})
							switch i {
							case 0:
								xMark := scaled(2)
								for d := -xMark; d <= xMark; d++ {
									dl.AddRectFilled(imgui.Vec2{X: center.X + d - 1, Y: center.Y + d - 1}, imgui.Vec2{X: center.X + d + 1, Y: center.Y + d + 1}, markCol)
									dl.AddRectFilled(imgui.Vec2{X: center.X + d - 1, Y: center.Y - d - 1}, imgui.Vec2{X: center.X + d + 1, Y: center.Y - d + 1}, markCol)
								}
							case 1:
								dl.AddRectFilled(imgui.Vec2{X: center.X - mark, Y: center.Y - 1}, imgui.Vec2{X: center.X + mark, Y: center.Y + 1}, markCol)
							case 2:
								arrowMark := scaled(2.5)
								inner := arrowMark / 4
								dl.PathClear()
								dl.PathLineTo(imgui.Vec2{X: center.X + arrowMark, Y: center.Y - arrowMark})
								dl.PathLineTo(imgui.Vec2{X: center.X + arrowMark, Y: center.Y + inner})
								dl.PathLineTo(imgui.Vec2{X: center.X - inner, Y: center.Y - arrowMark})
								dl.PathFillConvex(markCol)
								dl.PathClear()
								dl.PathLineTo(imgui.Vec2{X: center.X - arrowMark, Y: center.Y + arrowMark})
								dl.PathLineTo(imgui.Vec2{X: center.X - arrowMark, Y: center.Y - inner})
								dl.PathLineTo(imgui.Vec2{X: center.X + inner, Y: center.Y + arrowMark})
								dl.PathFillConvex(markCol)
							}
						}
						if clicked {
							switch i {
							case 0:
								win.SetShouldClose(true)
							case 1:
								C.glfwIconifyWindow(gw)
							case 2:
								if maximized {
									C.glfwRestoreWindow(gw)
								} else {
									C.glfwMaximizeWindow(gw)
								}
							}
						}
					}
				} else {
					for i, id := range []string{"##minimize", "##maximize", "##close"} {
						x := pos.X + size.X - btnW*float32(3-i)
						imgui.SetCursorScreenPos(imgui.Vec2{X: x, Y: pos.Y})
						clicked := imgui.InvisibleButton(id, imgui.Vec2{X: btnW, Y: barH})
						hovered := imgui.IsItemHovered()
						if hovered {
							dl.AddRectFilled(imgui.Vec2{X: x, Y: pos.Y}, imgui.Vec2{X: x + btnW, Y: pos.Y + barH}, g.ColorToUint(Ternary(i == 2, ThemeAccentRed, ThemeBg3)))
						}
						mx, my, r := snap(x+btnW/2), snap(pos.Y+barH/2), scaled(5)
						switch i {
						case 0:
							dl.AddLine(imgui.Vec2{X: mx - r, Y: my}, imgui.Vec2{X: mx + r, Y: my}, iconCol)
							if clicked {
								C.glfwIconifyWindow(gw)
							}
						case 1:
							dl.AddRect(imgui.Vec2{X: mx - r, Y: my - r}, imgui.Vec2{X: mx + r + 1, Y: my + r + 1}, iconCol)
							if clicked {
								if maximized {
									C.glfwRestoreWindow(gw)
								} else {
									C.glfwMaximizeWindow(gw)
								}
							}
						case 2:
							xCol := Ternary(hovered, g.ColorToUint(ThemeFg1), iconCol)
							for d := -r; d <= r; d++ {
								dl.AddRectFilled(imgui.Vec2{X: mx + d, Y: my + d}, imgui.Vec2{X: mx + d + 1, Y: my + d + 1}, xCol)
								dl.AddRectFilled(imgui.Vec2{X: mx + d, Y: my - d}, imgui.Vec2{X: mx + d + 1, Y: my - d + 1}, xCol)
							}
							if clicked {
								win.SetShouldClose(true)
							}
						}
					}
				}

				imgui.SetCursorPos(imgui.Vec2{X: start.X, Y: start.Y + barH - scaled(24)})
			}),
			fontRegular(20).style().To(
				renderHeader(),
				g.Dummy(0, scaled(16)),

				&CondWidget{
					GithubError != nil,
					func() g.Widget {
						return g.Column(
							renderCalloutCard(
								"GithubErrorCard",
								ThemeAccentRed,
								ThemeBgVisualRed,
								"Failed to fetch release info from GitHub",
								"Equicord build downloads are currently unavailable.",
								g.Column(
									g.Custom(func() {
										labelFont := fontRegular(20)
										linkFont := fontSemiBold(20)
										padX := scaled(3)
										padY := scaled(2)
										var rowH float32
										labelFont.style().To(g.Custom(func() {
											_, textH := g.CalcTextSize("Ag")
											rowH = textH + padY*2
										})).Build()
										renderAlignedRow(
											rowH,
											scaled(2),
											rowAlignedLabel("If this issue persists, visit", ThemeFg1, labelFont),
											rowAlignedLink("our support server", SupportUrl, linkFont, padX, padY, func() {
												g.OpenURL(SupportUrl)
											}),
											rowAlignedLabel("for help.", ThemeFg1, labelFont),
										).Build()
									}),
									g.Dummy(0, scaled(8)),
									fontSemiBold(18).style().To(
										accentButton("Retry", ThemeAccentRed, scaled(115), scaled(34), false, "Retry fetching release info from GitHub", retryGithubFetch),
									),
								),
							),
							g.Dummy(0, scaled(14)),
						)
					},
					nil,
				},

				renderInstaller(),
			),
		)

	g.PopStyleV(pushedVars)
}
