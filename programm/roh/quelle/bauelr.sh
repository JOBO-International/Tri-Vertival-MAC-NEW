#!/bin/bash
# bauelr.sh <Quellordner> <Zielordner> <g++> <ar>
Q=$1; Z=$2; CXX=$3; AR=$4
mkdir -p $Z/o; cd $Q
FILES=$(find src -name '*.cpp' ! -path '*integration*')
ls $FILES | xargs -P 8 -I{} sh -c 'o='$Z'/o/$(echo {} | tr / _).o; '$CXX' -O2 -std=gnu++14 -D_USE_MATH_DEFINES -w -I. -DLIBRAW_NOTHREADS -DUSE_X3FTOOLS -DLIBRAW_NODLL -c {} -o $o' 
$AR rcs $Z/libraw.a $Z/o/*.o
ls -la $Z/libraw.a
