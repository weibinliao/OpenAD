Option Explicit

Dim fso
Dim shell
Dim rootDir
Dim stopScript

Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("WScript.Shell")

rootDir = fso.GetParentFolderName(WScript.ScriptFullName)
stopScript = rootDir & "\scripts\stop-windows.bat"

If Not fso.FileExists(stopScript) Then
    MsgBox "Stop script not found: " & stopScript, vbCritical, "PermissionProtector"
    WScript.Quit 1
End If

shell.Run """" & stopScript & """", 0, False
