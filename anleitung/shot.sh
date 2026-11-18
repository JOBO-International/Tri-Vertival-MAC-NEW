#!/bin/bash
# shot.sh <bild-windows-pfad> <aktionen-bash> ; Aktionen nutzen "snap NAME"
export WINEDEBUG=-all WINEPREFIX=/root/.winewow
cd /tmp
BILD="$1"; AKT="$2"
xvfb-run -a -s "-screen 0 1400x900x24" bash -c "
snap() { import -window root /tmp/anl/v36/\$1.png; }
wine 'C:\\users\\root\\AppData\\Local\\Programs\\Vertical3\\Vertical3.exe' $BILD >/tmp/v3.log 2>&1 &
sleep 25
$AKT
wineserver -k"
