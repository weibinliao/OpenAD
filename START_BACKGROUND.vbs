Option Explicit

Dim fso
Dim shell
Dim rootDir
Dim powerShellExe
Dim startScript
Dim commandLine

Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("WScript.Shell")

rootDir = fso.GetParentFolderName(WScript.ScriptFullName)
powerShellExe = shell.ExpandEnvironmentStrings("%SystemRoot%") & "\System32\WindowsPowerShell\v1.0\powershell.exe"
startScript = rootDir & "\scripts\start-source-background.ps1"

If Not fso.FileExists(startScript) Then
    MsgBox "Background launcher not found: " & startScript, vbCritical, "PermissionProtector"
    WScript.Quit 1
End If

commandLine = """" & powerShellExe & """ -NoProfile -ExecutionPolicy Bypass -File """ & startScript & """ -ProjectRoot """ & rootDir & """"
shell.Run commandLine, 0, False
