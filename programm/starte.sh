#!/bin/bash
# Startet Vertical3.exe unter Wine, macht nach $2 Sekunden ein Bildschirmfoto nach $3
# $1 = Argument (Windows-Pfad), optional; GEOM=1400x900 als Umgebungsvariable
rm -rf /tmp/v3app/Vertical3.exe; mkdir -p /tmp/v3app; cp /home/claude/build/Vertical3.exe /tmp/v3app/
export WINEDEBUG=-all WINEPREFIX=/root/.winewow
cd /tmp
xvfb-run -a -s "-screen 0 ${GEOM:-1400x900}x24" bash -c "wine /tmp/v3app/Vertical3.exe $1 >/tmp/v3.log 2>&1 & sleep ${2:-20}; ${AKTION:-true}; import -window root $3; wineserver -k"
head -20 /tmp/v3.log
