#!/bin/sh
# redis-sentinel-notify.sh

MAIL_FROM=""
MAIL_TO=""

if [ $SENTINEL_NOTIFY_ENABLE = "1" ]; then
   if [ "$#" = "2" ] && ([ ${1} = "+sdown" ] || [ ${1} = "+odown" ] || [ ${1} = "+switch-master" ] || [ ${1} = "-sdown" ]); then
      mail_subject="Redis Notification"
      mail_body=`cat << EOB
============================================

Redis Notification Script called by Sentinel
Sentinel: $HOSTNAME

============================================

Event Type: ${1}
Event Description: ${2}

Check the redis status changed.
EOB`
    echo -e "${mail_body}" | mail -r "${MAIL_FROM}" -s "${mail_subject}" "${MAIL_TO}"
   fi
fi
