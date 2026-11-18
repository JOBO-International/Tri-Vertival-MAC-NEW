; Tri-Vertical Filter – Setup für das Zusatzprogramm (Inno Setup 6.4 oder neuer, auch 7)
; Wird über „Setups bauen.bat“ im Bausatz-Ordner gebaut.
; Installiert in den Programmordner von Tri-Vertical (intern „Vertical3“).

#define AppVersion "1.5"
#define Braucht "2.8"
#define Kurz "Filter"

[Setup]
AppId=Vertical3-Filter
AppName=Tri-Vertical Filter
AppVersion={#AppVersion}
AppVerName=Tri-Vertical Filter {#AppVersion}
AppPublisher=Marcus (privat)
AppCopyright=Privater Gebrauch
VersionInfoVersion={#AppVersion}.0
VersionInfoProductName=Tri-Vertical Filter
VersionInfoDescription=Tri-Vertical Filter Setup
DefaultDirName={code:Hauptordner}
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableWelcomePage=no
DisableReadyPage=yes
DirExistsWarning=no
UsePreviousAppDir=no
PrivilegesRequired=lowest
OutputDir=..\Setups
OutputBaseFilename=Tri-Vertical-Filter-Setup-{#AppVersion}
SetupIconFile=vertical3.ico
UninstallFilesDir={app}\zusatz\filter-deinst
UninstallDisplayIcon={app}\Vertical3-Filter.exe
UninstallDisplayName=Tri-Vertical Filter (Zusatzprogramm)
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
de.WelcomeLabel1=Zusatzprogramm Tri-Vertical Filter
de.WelcomeLabel2=Dieser Assistent installiert das Zusatzprogramm Tri-Vertical Filter {#AppVersion}.%n%n55 Bildlooks – Farbe, Kamera-Looks, Farbnegativ- und Diafilme, Sofortbild, Schwarzweißfilme sowie Edeldrucke wie Palladium, Platin und Cyanotypie – mit Live-Vorschau und Vorher/Nachher-Vergleich, Feinabstimmung (Belichtung, Kontrast, Wärme, Klarheit, Vignette, Körnung …) und eigene Filter.%n%nVoraussetzung: Tri-Vertical ab Version {#Braucht}.
de.FinishedHeadingLabel=Tri-Vertical Filter ist installiert
de.FinishedLabel=Bitte Tri-Vertical neu starten. Danach erscheint „Filter“ im Menü Datei und im Bedienfeld (Tastenkürzel Strg+F).

[InstallDelete]
; Verknüpfung älterer Versionen
Type: files; Name: "{userprograms}\Vertical3\Vertical3 Filter.lnk"

[Files]
Source: "Vertical3-Filter.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "LIESMICH-Filter.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "zusatz\filter.json"; DestDir: "{app}\zusatz"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\Vertical3\Tri-Vertical Filter"; Filename: "{app}\Vertical3-Filter.exe"

[Run]
Filename: "{app}\LIESMICH-Filter.txt"; Description: "Kurzanleitung anzeigen"; Flags: shellexec postinstall skipifsilent unchecked

[UninstallDelete]
Type: files; Name: "{userprograms}\Vertical3\Tri-Vertical Filter.lnk"

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
