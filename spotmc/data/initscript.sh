#!/bin/bash
# chkconfig: 3 99 10
# processname: dummy-smc-stopper

start() {
	touch /var/lock/subsys/dummy-smc-stopper
}

stop() {
	rm -f /var/lock/subsys/dummy-smc-stopper
	killall spotmc
	echo "dummy-smc-stopper: waiting 30 seconds"
	sleep 30
}

case "$1" in
  start)
        start
        ;;
  stop)
        stop
        ;;
esac
