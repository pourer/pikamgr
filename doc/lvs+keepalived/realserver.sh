#!/bin/bash

VIPS[0]=192.168.107.181
VIPS[1]=192.168.107.182

source /etc/rc.d/init.d/functions

case "$1" in
start)
       i=0
       for vip in ${VIPS[@]}
       do
         ifconfig lo:$i $vip netmask 255.255.255.255 broadcast $vip
         /sbin/route add -host $vip dev lo:$i
         let i++
       done
       echo "1" >/proc/sys/net/ipv4/conf/lo/arp_ignore
       echo "2" >/proc/sys/net/ipv4/conf/lo/arp_announce
       echo "1" >/proc/sys/net/ipv4/conf/all/arp_ignore
       echo "2" >/proc/sys/net/ipv4/conf/all/arp_announce
       echo "RealServer Start OK"
       ;;
stop)
       i=0
       for vip in ${VIPS[@]}
       do
         ifconfig lo:$i down
         route del $vip >/dev/null 2>&1
         let i++
       done
       echo "0" >/proc/sys/net/ipv4/conf/lo/arp_ignore
       echo "0" >/proc/sys/net/ipv4/conf/lo/arp_announce
       echo "0" >/proc/sys/net/ipv4/conf/all/arp_ignore
       echo "0" >/proc/sys/net/ipv4/conf/all/arp_announce
       echo "RealServer Stoped"
       ;;
       *)
       echo "Usage: $0 {start|stop}"
       exit 1
esac
exit 0
