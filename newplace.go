package main

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultPlaceFile = "place.rbxlx"

// Minimal place Studio will open. Script services exist so Sync to… has targets.
const minimalPlaceXML = `<?xml version="1.0" encoding="utf-8"?>
<roblox xmlns:xmime="http://www.w3.org/2005/05/xmlmime" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="http://www.roblox.com/roblox.xsd" version="4">
	<Item class="Workspace" referent="RBXWORKSPACE">
		<Properties>
			<string name="Name">Workspace</string>
		</Properties>
		<Item class="Part" referent="RBXBASEPLATE">
			<Properties>
				<string name="Name">Baseplate</string>
				<bool name="Anchored">true</bool>
				<bool name="Locked">true</bool>
				<Vector3 name="Size">
					<X>512</X>
					<Y>16</Y>
					<Z>512</Z>
				</Vector3>
				<CoordinateFrame name="CFrame">
					<X>0</X><Y>-8</Y><Z>0</Z>
					<R00>1</R00><R01>0</R01><R02>0</R02>
					<R10>0</R10><R11>1</R11><R12>0</R12>
					<R20>0</R20><R21>0</R21><R22>1</R22>
				</CoordinateFrame>
			</Properties>
		</Item>
		<Item class="SpawnLocation" referent="RBXSPAWN">
			<Properties>
				<string name="Name">SpawnLocation</string>
				<bool name="Anchored">true</bool>
				<float name="Duration">0</float>
				<Vector3 name="Size">
					<X>12</X>
					<Y>1</Y>
					<Z>12</Z>
				</Vector3>
				<CoordinateFrame name="CFrame">
					<X>0</X><Y>0.5</Y><Z>0</Z>
					<R00>1</R00><R01>0</R01><R02>0</R02>
					<R10>0</R10><R11>1</R11><R12>0</R12>
					<R20>0</R20><R21>0</R21><R22>1</R22>
				</CoordinateFrame>
			</Properties>
		</Item>
	</Item>
	<Item class="Lighting" referent="RBXLIGHTING">
		<Properties>
			<string name="Name">Lighting</string>
		</Properties>
	</Item>
	<Item class="ReplicatedFirst" referent="RBXRF">
		<Properties>
			<string name="Name">ReplicatedFirst</string>
		</Properties>
	</Item>
	<Item class="ReplicatedStorage" referent="RBXRS">
		<Properties>
			<string name="Name">ReplicatedStorage</string>
		</Properties>
	</Item>
	<Item class="ServerScriptService" referent="RBXSSS">
		<Properties>
			<string name="Name">ServerScriptService</string>
		</Properties>
	</Item>
	<Item class="ServerStorage" referent="RBXSS">
		<Properties>
			<string name="Name">ServerStorage</string>
		</Properties>
	</Item>
	<Item class="StarterGui" referent="RBXSG">
		<Properties>
			<string name="Name">StarterGui</string>
		</Properties>
	</Item>
	<Item class="StarterPack" referent="RBXSP">
		<Properties>
			<string name="Name">StarterPack</string>
		</Properties>
	</Item>
	<Item class="StarterPlayer" referent="RBXSTARTERPLAYER">
		<Properties>
			<string name="Name">StarterPlayer</string>
		</Properties>
		<Item class="StarterPlayerScripts" referent="RBXSPS">
			<Properties>
				<string name="Name">StarterPlayerScripts</string>
			</Properties>
		</Item>
		<Item class="StarterCharacterScripts" referent="RBXSCS">
			<Properties>
				<string name="Name">StarterCharacterScripts</string>
			</Properties>
		</Item>
	</Item>
	<Item class="SoundService" referent="RBXSOUND">
		<Properties>
			<string name="Name">SoundService</string>
		</Properties>
	</Item>
</roblox>
`

func createNewPlace(name, file string) place {
	existing := loadPlace()
	if existing.PlaceID != 0 || existing.UniverseID != 0 {
		fail(exitError, "ERROR: place.json already points at Roblox place %d (universe %d).\nOmit --new to open that place, or clear placeId/universeId first.",
			existing.PlaceID, existing.UniverseID)
	}

	path := strings.TrimSpace(file)
	if path == "" {
		path = strings.TrimSpace(existing.LocalPlaceFile)
	}
	if path == "" {
		path = defaultPlaceFile
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		fail(exitError, "ERROR: %v", err)
	}
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".rbxl", ".rbxlx":
	default:
		abs += ".rbxlx"
	}

	if st, err := os.Stat(abs); err == nil && !st.IsDir() {
		info("Reusing existing %s", abs)
	} else {
		if err := os.WriteFile(abs, []byte(minimalPlaceXML), 0o644); err != nil {
			fail(exitError, "ERROR: could not create place file:\n  %s\n  %v", abs, err)
		}
		info("Created local place %s", abs)
	}
	if name == "" {
		name = existing.Name
	}
	p := place{
		LocalPlaceFile: repoRel(abs),
		Name:           name,
	}
	savePlace(p)
	if name != "" {
		info("New game %q — File → Publish to Roblox when you want a cloud place id.", name)
	} else {
		info("New local place — File → Publish to Roblox when you want a cloud place id.")
	}
	return p
}
