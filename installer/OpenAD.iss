#ifndef MyAppVersion
  #define MyAppVersion "1.0.0"
#endif
#ifndef SourceDir
  #error SourceDir must be provided by build-windows-installer.ps1
#endif
#ifndef OutputDir
  #error OutputDir must be provided by build-windows-installer.ps1
#endif

#define MyAppName "OpenAD"
#define MyAppPublisher "OpenAD Project"
#define MyAppExeName "OpenAD.exe"
#define MyAppId "{{2D72B63F-07BA-4CB1-9794-B4A558B46AB1}"

[Setup]
AppId={#MyAppId}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL=https://github.com/weibinliao/OpenAD
AppSupportURL=https://github.com/weibinliao/OpenAD/issues
AppUpdatesURL=https://github.com/weibinliao/OpenAD/releases
VersionInfoVersion={#MyAppVersion}.0
VersionInfoCompany={#MyAppPublisher}
VersionInfoDescription=OpenAD Windows Installer
VersionInfoProductName={#MyAppName}
DefaultDirName={localappdata}\Programs\OpenAD
DefaultGroupName=OpenAD
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=OpenAD
SetupIconFile=..\apps\desktop-win\OpenAD.ico
LicenseFile={#SourceDir}\LICENSE
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\{#MyAppExeName}
AppMutex=Local\OpenAD.Desktop.7E29C1B4-7D94-4B0D-BCB2-0B3CF582B33A.SingleInstance.v1
SetupLogging=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs; Excludes: "*.pdb,*.db,*.db-shm,*.db-wal,*.sqlite,*.sqlite3,*.log,*.pid,.env,.env.*,*.p12,*.pfx,*.pem,*.key"

[Icons]
Name: "{group}\OpenAD"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"
Name: "{group}\{cm:UninstallProgram,OpenAD}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\OpenAD"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,OpenAD}"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent
