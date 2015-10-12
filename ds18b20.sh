#!/bin/bash

W1DIR="/sys/bus/w1/devices"
WORKDIR="/tmp/ds18b20"
MAX=6
THINGSPEAK="${WORKDIR}/thingspeak"
SENSORS="${WORKDIR}/sensors"
NAME=$(basename $0 .sh)
LOCKFILE="/var/lock/${NAME}.lock"
PIDFILE="/var/run/${NAME}.pid"
LOCKFD="100"
SLEEP=10
SENDSLEEP=60
URL="https://api.thingspeak.com/update"
API_KEY="key"
LOG="/var/log/${NAME}.log"

function init() {
	[ ! -d "${WORKDIR}" ] && mkdir -p "${WORKDIR}"
	[ ! -d "${SENSORS}" ] && mkdir -p "${SENSORS}"
	[ ! -d "${THINGSPEAK}" ] && mkdir -p "${THINGSPEAK}"
	getcount >/dev/null
}

function discovery() {
	for slave in $(cat "${W1DIR}/w1_bus_master1/w1_master_slaves"); do
		if [ ! -h "${SENSORS}/${slave}" ]; then
			field=$(countinc)
			ln -s "${W1DIR}/${slave}" "${SENSORS}/${slave}"
			ln -s "${W1DIR}/${slave}" "${THINGSPEAK}/field${field}"
		fi
	done
}

function getfieldname() {
	key=$1
	if [ -n "${key}" ]; then
		for link in $(find "${THINGSPEAK}" -type l); do 
			sk=$(basename $(readlink "${link}"))
			if [ "${key}" == "${sk}" ]; then
				echo "$(basename "${link}")"
				break
			fi
		done
	fi
}

function getsensors() {
	for sensor in ${SENSORS}/*; do
		echo "$(basename "${sensor}")"
	done
}

function getcount() {
	count=0
	if [ -e "${WORKDIR}/count" ]; then
		count=$(cat "${WORKDIR}/count")
	else
		echo "${count}" > "${WORKDIR}/count"
	fi
	echo "$count"
}

function countinc() {
	count=0
	if [ -e "${WORKDIR}/count" ]; then
		count=$(cat "${WORKDIR}/count")
	fi
	let count=${count}+1
	echo "${count}" > "${WORKDIR}/count"
	echo "${count}"
}

function getdstemp() {
	key=$1
	file="${W1DIR}/${key}/w1_slave"
	
	if [ -n "${key}" -a -e ${file} ];then
		data=$(cat "${file}")
		CRC=$(echo $data | head -n1 | cut -d" " -f 9)
		STATE=$(echo $data | head -n1 | cut -d" " -f 12)

		if [ "${STATE}" == "YES" ]; then
			CHECK=$(echo $data | tail -n1 | cut -d" " -f 9)
			if [ "${CHECK}" == "${CRC}" ]; then
				TEMP=$(echo $data | tail -n1 | cut -d"=" -f 3)
				TEMP=$(echo "scale=2; $TEMP/1000" | bc)
				fpush "${key}" "${TEMP}"
			fi		
		fi
	fi
}

function fsize() {
	count=0
	key=$1
	file="${WORKDIR}/${key}.stack"
	if [ -n "${key}" -a -e "${file}" ]; then
		count=$(cat "${file}" | wc -l)
	fi
	echo $count
}

function fpop() {
	key=$1
	file="${WORKDIR}/${key}.stack"
	if [ -n "${key}" -a -e "${file}" ]; then
		sed '1d' -i "${file}"
	fi
}

function ftruncate() {
	key=$1
	file="${WORKDIR}/${key}.stack"
	size=$(fsize "${key}")
	if [ -n "${key}" -a -e "${file}" ]; then
		while [ "${size}" -gt "${MAX}" ]; do
			fpop "${key}"
			size=$(fsize "${key}")
		done
	fi
}

function fpush() {
	key=$1
	val=$2
	file="${WORKDIR}/${key}.stack"
	size=$(fsize "${key}")
	
	if [ -n "${key}" -a -n "${val}" ]; then
		if [ "${size}" -ge "$MAX" ]; then
			fpop "${key}"
		fi
		echo "${val}" >> "${file}"
	fi
}

function favg() {
	key=$1
	file="${WORKDIR}/${key}.stack"
	avg=0
	sum=0
	count=0
	if [ -n "${key}" -a -e "${file}" ]; then
		for line in $(cat "${file}"); do
			sum=$(echo "scale=2; ${sum}+${line}" | bc)
			let count=$count+1
		done
		if [ "${count}" -gt "0" ]; then
			avg=$(echo "scale=2; ${sum}/${count}" | bc)
		fi
		echo "${avg}"
	fi
}

function start() {
	init
	discovery
	eval "exec ${LOCKFD}>${LOCKFILE}"
	
	if flock -n ${LOCKFD} ; then
		
		cd /
		exec >${LOG}
		exec 2>${LOG}
		exec </dev/null
		
		(
			trap '[ -n "$(jobs -pr)" ] && kill $(jobs -pr); flock -u ${LOCKFD}' INT QUIT TERM EXIT
			echo ${BASHPID}>${PIDFILE}
			(
				while [ 1 ]; do
					for sensor in $(getsensors); do 
						getdstemp "${sensor}" &
					done

					sleep "${SLEEP}"
				done
			) &

			(
				while [ 1 ]; do
					get="api_key=$API_KEY"
					for sensor in $(getsensors); do
						field=$(getfieldname "${sensor}")
						temp=$(favg "${sensor}")
						get="${get}&${field}=${temp}"
					done
					curl --connect-timeout "$((${SENDSLEEP} - 1))" -k --data "${get}" ${URL} >/dev/null 2>&1 &
					#echo "$(date) curl --connect-timeout $((${SENDSLEEP} - 1)) -k --data \"${get}\" ${URL}"
					sleep "${SENDSLEEP}"
				done
			) &

			wait
		) &
	else
		PID=$(cat ${PIDFILE})
		echo "Daemon already running with pid=$PID"
	fi
}

function reload() {
	discovery
}

function stop() {
	pid=$(cat "${PIDFILE}")
	kill "${pid}"
}

case "$1" in
	"start")
		start
		;;
	"reload")
		reload
		;;
	"stop")
		stop
		;;
	"restart")
		stop
		sleep 1
		start
		;;
esac
