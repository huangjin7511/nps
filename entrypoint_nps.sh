#!/bin/sh

if [ ! -d /conf ]; then
    mkdir -p /conf
fi

if [ ! -f /conf/nps.conf ]; then
    cp /nps.conf.sample /conf/nps.conf
fi

if [ ! -f /conf/geoip.dat ]; then
    cp /geoip.dat.sample /conf/geoip.dat
fi

if [ -f /geosite.dat.sample ] && [ ! -f /conf/geosite.dat ]; then
    cp /geosite.dat.sample /conf/geosite.dat
fi

/nps service
