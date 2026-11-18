; Tri-Vertical – Setup (Inno Setup 6.4 oder neuer, auch Inno Setup 7)
; Bauen: „Setups bauen.bat“ im übergeordneten Ordner doppelklicken.
; Intern heißen Programmdatei, Ordner und Registry weiterhin „Vertical3“,
; damit Updates, Einstellungen und Zusatzprogramme weiterlaufen.

#define AppVersion "3.9.1"

[Setup]
AppId=Tri-Vertical
AppName=Tri-Vertical
AppVersion={#AppVersion}
AppVerName=Tri-Vertical {#AppVersion}
AppPublisher=Marcus (privat)
AppCopyright=Privater Gebrauch
VersionInfoVersion={#AppVersion}.0
VersionInfoProductName=Tri-Vertical
VersionInfoDescription=Tri-Vertical Setup
DefaultDirName={code:InstallOrdner}
UsePreviousAppDir=no
DisableDirPage=no
DisableWelcomePage=no
DirExistsWarning=no
DefaultGroupName=Vertical3
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=..\Setups
OutputBaseFilename=Tri-Vertical-Setup-{#AppVersion}
SetupIconFile=vertical3.ico
UninstallDisplayIcon={app}\Vertical3.exe
UninstallDisplayName=Tri-Vertical – Perspektivkorrektur
WizardStyle=modern
WizardImageFile=bilder\gross1.bmp,bilder\gross2.bmp
WizardSmallImageFile=bilder\klein1.bmp,bilder\klein2.bmp
Compression=lzma2/max
SolidCompression=yes
ChangesAssociations=yes
CloseApplications=yes
RestartApplications=no
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0

[Languages]
Name: "de"; MessagesFile: "compiler:Languages\German.isl"

[Messages]
de.WelcomeLabel2=Dieser Assistent installiert Tri-Vertical {#AppVersion} (bisher Vertical3) – Perspektivkorrektur für Fotos.%n%nEs wird nichts weiter benötigt: keine Internetverbindung, keine Administratorrechte.%n%nEine vorhandene ältere Version wird automatisch ersetzt; Einstellungen, eigene Größen, Kameraprofile und Zusatzprogramme bleiben erhalten.

[InstallDelete]
; Reste der alten Python-Version 1.x
Type: filesandordirs; Name: "{app}\python"
Type: filesandordirs; Name: "{app}\wheels"
Type: filesandordirs; Name: "{app}\__pycache__"
Type: filesandordirs; Name: "{app}\_sicherung"
Type: files; Name: "{app}\vertical3.py"
Type: files; Name: "{app}\vertical3_core.py"
Type: files; Name: "{app}\vertical3_update.py"
Type: files; Name: "{app}\requirements.txt"
Type: files; Name: "{app}\kameraprofile\leica_d-lux.json"
; Verknüpfungen älterer Versionen
Type: files; Name: "{userdesktop}\Vertical3.lnk"
Type: files; Name: "{userprograms}\Vertical3\Vertical3.lnk"
Type: files; Name: "{userprograms}\Vertical3\Vertical3 deinstallieren.lnk"

[Files]
Source: "Vertical3.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "vertical3.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "ANLEITUNG.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "version.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "LIZENZEN-RAW.txt"; DestDir: "{app}"; Flags: ignoreversion
; Kameraprofile: nur ersetzen, wenn die mitgelieferte Datei neuer ist
Source: "kameraprofile\*.json"; DestDir: "{app}\kameraprofile"; Flags: comparetimestamp

[Icons]
Name: "{group}\Tri-Vertical"; Filename: "{app}\Vertical3.exe"; Comment: "Perspektivkorrektur"
Name: "{group}\Anleitung"; Filename: "{app}\ANLEITUNG.txt"
Name: "{group}\Kameraprofile"; Filename: "{app}\kameraprofile"
Name: "{group}\Tri-Vertical deinstallieren"; Filename: "{uninstallexe}"
Name: "{userdesktop}\Tri-Vertical"; Filename: "{app}\Vertical3.exe"; Comment: "Perspektivkorrektur"

[Registry]
; Programmordner – daran finden Updates und Zusatzprogramme die Installation
Root: HKCU; Subkey: "Software\Vertical3"; ValueType: string; ValueName: "InstallDir"; ValueData: "{app}"; Flags: uninsdeletekey
; Dateityp .v3cam (Kameraprofil)
Root: HKCU; Subkey: "Software\Classes\.v3cam"; ValueType: string; ValueData: "Vertical3.Kameraprofil"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\Vertical3.Kameraprofil"; ValueType: string; ValueData: "Tri-Vertical-Kameraprofil"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\Vertical3.Kameraprofil\DefaultIcon"; ValueType: string; ValueData: "{app}\vertical3.ico"
Root: HKCU; Subkey: "Software\Classes\Vertical3.Kameraprofil\shell\open"; ValueType: string; ValueData: "In Tri-Vertical importieren"
Root: HKCU; Subkey: "Software\Classes\Vertical3.Kameraprofil\shell\open\command"; ValueType: string; ValueData: """{app}\Vertical3.exe"" ""%1"""
; Dateityp .v3update (Programm-Update)
Root: HKCU; Subkey: "Software\Classes\.v3update"; ValueType: string; ValueData: "Vertical3.Update"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\Vertical3.Update"; ValueType: string; ValueData: "Tri-Vertical-Update"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\Vertical3.Update\DefaultIcon"; ValueType: string; ValueData: "{app}\vertical3.ico"
Root: HKCU; Subkey: "Software\Classes\Vertical3.Update\shell\open"; ValueType: string; ValueData: "Update installieren"
Root: HKCU; Subkey: "Software\Classes\Vertical3.Update\shell\open\command"; ValueType: string; ValueData: """{app}\Vertical3.exe"" ""%1"""
; „Öffnen mit → Tri-Vertical“ für Fotos
Root: HKCU; Subkey: "Software\Classes\Applications\Vertical3.exe"; ValueType: string; ValueName: "FriendlyAppName"; ValueData: "Tri-Vertical"; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\Applications\Vertical3.exe\shell\open\command"; ValueType: string; ValueData: """{app}\Vertical3.exe"" ""%1"""
Root: HKCU; Subkey: "Software\Classes\Applications\Vertical3.exe\SupportedTypes"; ValueType: string; ValueName: ".jpg"; ValueData: ""
Root: HKCU; Subkey: "Software\Classes\Applications\Vertical3.exe\SupportedTypes"; ValueType: string; ValueName: ".jpeg"; ValueData: ""
Root: HKCU; Subkey: "Software\Classes\Applications\Vertical3.exe\SupportedTypes"; ValueType: string; ValueName: ".heic"; ValueData: ""
Root: HKCU; Subkey: "Software\Classes\Applications\Vertical3.exe\SupportedTypes"; ValueType: string; ValueName: ".tif"; ValueData: ""
Root: HKCU; Subkey: "Software\Classes\Applications\Vertical3.exe\SupportedTypes"; ValueType: string; ValueName: ".png"; ValueData: ""
Root: HKCU; Subkey: "Software\Classes\.jpg\OpenWithProgids"; ValueType: string; ValueName: "Applications\Vertical3.exe"; ValueData: ""; Flags: uninsdeletevalue
Root: HKCU; Subkey: "Software\Classes\.jpeg\OpenWithProgids"; ValueType: string; ValueName: "Applications\Vertical3.exe"; ValueData: ""; Flags: uninsdeletevalue
Root: HKCU; Subkey: "Software\Classes\.heic\OpenWithProgids"; ValueType: string; ValueName: "Applications\Vertical3.exe"; ValueData: ""; Flags: uninsdeletevalue

[Run]
Filename: "{app}\Vertical3.exe"; Description: "Tri-Vertical jetzt starten"; Flags: nowait postinstall skipifsilent
Filename: "{app}\ANLEITUNG.txt"; Description: "Anleitung anzeigen"; Flags: shellexec postinstall skipifsilent unchecked

[UninstallDelete]
; alles im Programmordner (auch Zusatzprogramme und Updates), Startmenü-Ordner
Type: filesandordirs; Name: "{app}"
Type: filesandordirs; Name: "{userprograms}\Vertical3"
Type: files; Name: "{userdesktop}\Vertical3.lnk"

[Code]
const
  Deinst = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\';
var
  Neuinstallation: Boolean;

{ Programmordner einer vorhandenen Installation (auch der alten Setups) übernehmen }
function InstallOrdner(Param: String): String;
begin
  if not (RegQueryStringValue(HKCU, 'Software\Vertical3', 'InstallDir', Result) and DirExists(Result)) then
    Result := ExpandConstant('{localappdata}\Programs\Vertical3');
end;

{ Ist Tri-Vertical schon installiert, wird ohne Ordnerauswahl in denselben Ordner installiert }
function ShouldSkipPage(PageID: Integer): Boolean;
var
  Alt: String;
begin
  Result := (PageID = wpSelectDir) and RegQueryStringValue(HKCU, 'Software\Vertical3', 'InstallDir', Alt) and DirExists(Alt);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Code: Integer;
begin
  if CurStep = ssInstall then
  begin
    Neuinstallation := not FileExists(ExpandConstant('{app}\Vertical3.exe'));
    { alte Python-Version still entfernen }
    if FileExists(ExpandConstant('{app}\python-setup.exe')) then
    begin
      Exec(ExpandConstant('{app}\python-setup.exe'), '/quiet /uninstall', '', SW_HIDE, ewWaitUntilTerminated, Code);
      DeleteFile(ExpandConstant('{app}\python-setup.exe'));
    end;
  end;
  if CurStep = ssPostInstall then
  begin
    { erster Start nach einer Neuinstallation im Design „Hell“ }
    if Neuinstallation then
      SaveStringToFile(ExpandConstant('{app}\erststart.txt'), '1', False);
    { Eintrag und Deinstallationsprogramm des alten NSIS-Setups entfernen,
      damit Tri-Vertical unter „Installierte Apps“ nur einmal erscheint }
    RegDeleteKeyIncludingSubkeys(HKCU, Deinst + 'Vertical3');
    DeleteFile(ExpandConstant('{app}\uninstall.exe'));
  end;
end;

{ Schlüssel in beiden Registry-Ansichten löschen (die Zusatzprogramm-Setups sind 32-Bit) }
procedure SchluesselWeg(Name: String);
begin
  { bisherige Setups: "Vertical3-Filter", Inno-Setup-Setups: "Vertical3-Filter_is1" }
  RegDeleteKeyIncludingSubkeys(HKCU, Deinst + Name);
  RegDeleteKeyIncludingSubkeys(HKCU, Deinst + Name + '_is1');
  if IsWin64 then
  begin
    RegDeleteKeyIncludingSubkeys(HKCU32, Deinst + Name);
    RegDeleteKeyIncludingSubkeys(HKCU32, Deinst + Name + '_is1');
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
  begin
    { Zusatzprogramme liegen im Programmordner und werden mit entfernt }
    SchluesselWeg('Vertical3-Drucken');
    SchluesselWeg('Vertical3-Filter');
    SchluesselWeg('Vertical3-KI');
    SchluesselWeg('Vertical3-Rahmen');
    SchluesselWeg('Vertical3-Neigung');
    SchluesselWeg('Vertical3-Belichter');
    SchluesselWeg('Vertical3');
  end;
end;
