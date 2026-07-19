package events

var allowed = map[string]struct{}{
	"start": {}, "platformDetected": {}, "openSettings": {}, "openSet": {}, "openFolder": {}, "openEditor": {}, "openTobiiCalibration": {},
	"cardClick": {}, "toggleOutputLine": {}, "toggleGazeLock": {}, "share": {}, "move": {}, "trash": {}, "editorAddImage": {}, "editorAddAudio": {},
	"settingsToggleEyeExit": {}, "settingsToggleEyeChoose": {}, "settingsToggleEyeActivation": {}, "settingsToggleEyePagination": {},
	"settingsToggleKeyboardActivation": {}, "settingsToggleJoystickActivation": {}, "settingsToggleTypeSound": {}, "settingsToggleMouseActivation": {},
	"settingsTogglePageTurnMode": {}, "settingsToggleEyeScale": {}, "settingsSetTimeout": {}, "settingsToggleAnimation": {},
	"tobiiCalibrationStart": {}, "tobiiCalibrationPoint": {}, "tobiiCalibrationFinish": {}, "tobiiCalibrationCancel": {},
	"tobiiCalibrationError": {}, "tobiiCalibrationApplySaved": {}, "tobiiCalibrationApplySavedResult": {}, "tobiiCalibrationUnavailable": {},
	"updateAvailable": {}, "updateDownloaded": {}, "updateError": {}, "updateInstallConfirmed": {}, "deploySmoke": {},
}

func Allowed(name string) bool {
	_, ok := allowed[name]
	return ok
}
