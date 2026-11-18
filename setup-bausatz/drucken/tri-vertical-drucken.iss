; Tri-Vertical Drucken – Setup für das Zusatzprogramm (Inno Setup 6.4 oder neuer, auch 7)
; Wird über „Setups bauen.bat“ im Bausatz-Ordner gebaut.
; Installiert in den Programmordner von Tri-Vertical (intern „Vertical3“).

#define AppVersion "2.2"
#define Braucht "2.8"
#define Kurz "Drucken"

[Setup]
AppId=Vertical3-Drucken
AppName=Tri-Vertical Drucken
AppVersion={#AppVersion}
AppVerName=Tri-Vertical Drucken {#AppVersion}
AppPublisher=Marcus (privat)
AppCopyright=Privater Gebrauch
VersionInfoVersion={#AppVersion}.0
VersionInfoProductName=Tri-Vertical Drucken
VersionInfoDescription=Tri-Vertical Drucken Setup
DefaultDirName={code:Hauptordner}
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableWelcomePage=no
DisableReadyPage=yes
DirExistsWarning=no
UsePreviousAppDir=no
PrivilegesRequired=lowest
OutputDir=..\Setups
OutputBaseFilename=Tri-Vertical-Drucken-Setup-{#AppVersion}
SetupIconFile=vertical3.ico
UninstallFilesDir={app}\zusatz\drucken-deinst
UninstallDisplayIcon={app}\Vertical3-Druck.exe
UninstallDisplayName=Tri-Vertical Drucken (Zusatzprogramm)
WizardStyle=modern
WizardImageFile=bilder\gross1.bmp,bilder\gross2.bmp
WizardSmallImageFile=bilder\klein1.bmp,bilder\klein2.bmp
Compression=lzma2/max
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0

[Languages]
Name: "de"; MessagesFile: "compiler:Languages\German.isl"

[Messages]
de.WelcomeLabel1=Zusatzprogramm Tri-Vertical Drucken
de.WelcomeLabel2=Dieser Assistent installiert das Zusatzprogramm Tri-Vertical Drucken {#AppVersion}.%n%nDrucken mit Vorschau, Papierformaten aus dem Druckertreiber (auch randlos), Bildgrößen von 9×13 bis 30×45 cm und eigenen Größen.%n%nRahmen und Beschriftung gibt es im eigenen Zusatzprogramm „Tri-Vertical Rahmen“.%n%nVoraussetzung: Tri-Vertical ab Version {#Braucht}.
de.FinishedHeadingLabel=Tri-Vertical Drucken ist installiert
de.FinishedLabel=Bitte Tri-Vertical neu starten. Danach erscheint „Drucken“ im Menü Datei und im Bedienfeld (Tastenkürzel Strg+P).

[InstallDelete]
; Verknüpfung älterer Versionen
Type: files; Name: "{userprograms}\Vertical3\Vertical3 Drucken.lnk"

[Files]
Source: "Vertical3-Druck.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "LIESMICH-Drucken.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "zusatz\drucken.json"; DestDir: "{app}\zusatz"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\Vertical3\Tri-Vertical Drucken"; Filename: "{app}\Vertical3-Druck.exe"

[Run]
Filename: "{app}\LIESMICH-Drucken.txt"; Description: "Kurzanleitung anzeigen"; Flags: shellexec postinstall skipifsilent unchecked

[UninstallDelete]
Type: files; Name: "{userprograms}\Vertical3\Tri-Vertical Drucken.lnk"

[Code]
const
  Deinst = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\';

{ Versionsnummern wie "3.8.4" vergleichen: -1 kleiner, 0 gleich, 1 größer }
function VersionVergleich(A, B: String): Integer;
var
  PA, PB, NA, NB: Integer;
begin
  Result := 0;
  while (Result = 0) and ((A <> '') or (B <> '')) do
  begin
    PA := Pos('.', A); if PA = 0 then PA := Length(A) + 1;
    PB := Pos('.', B); if PB = 0 then PB := Length(B) + 1;
    NA := StrToIntDef(Copy(A, 1, PA - 1), 0);
    NB := StrToIntDef(Copy(B, 1, PB - 1), 0);
    if NA < NB then Result := -1 else if NA > NB then Result := 1;
    Delete(A, 1, PA); Delete(B, 1, PB);
  end;
end;

function Hauptordner(Param: String): String;
begin
  if not RegQueryStringValue(HKCU, 'Software\Vertical3', 'InstallDir', Result) then
    Result := '';
end;

{ Tri-Vertical muss installiert und aktuell genug sein }
function InitializeSetup(): Boolean;
var
  Ordner: String;
  Ver: AnsiString;
begin
  Result := False;
  Ordner := Hauptordner('');
  if (Ordner = '') or not FileExists(Ordner + '\Vertical3.exe') then
  begin
    MsgBox('Tri-Vertical wurde auf diesem Computer nicht gefunden.' + #13#10#13#10 +
      'Bitte zuerst Tri-Vertical installieren und dieses Setup danach erneut starten.', mbInformation, MB_OK);
    Exit;
  end;
  if not LoadStringFromFile(Ordner + '\version.txt', Ver) then
    Ver := '0';
  if VersionVergleich(Trim(String(Ver)), '{#Braucht}') < 0 then
  begin
    MsgBox('Installiert ist Tri-Vertical ' + Trim(String(Ver)) + '.' + #13#10#13#10 +
      'Das Zusatzprogramm braucht Tri-Vertical ab Version {#Braucht}. Bitte zuerst Tri-Vertical aktualisieren und dieses Setup danach erneut starten.',
      mbInformation, MB_OK);
    Exit;
  end;
  Result := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    { Eintrag und Deinstallationsprogramm des bisherigen Setups entfernen }
    RegDeleteKeyIncludingSubkeys(HKCU, Deinst + 'Vertical3-{#Kurz}');
    DeleteFile(ExpandConstant('{app}\zusatz\Vertical3-{#Kurz}-deinstallieren.exe'));
  end;
end;
