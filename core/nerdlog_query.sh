#/bin/bash

# Arguments:
#
# --from, --to: time in the format "2006-01-02-15:04".

# Those numbers are supposed to go up as the query progresses; the Go app
# will then be able to tell which node is the slowest and show info for it.
STAGE_INDEX_FULL=1
STAGE_INDEX_APPEND=2
STAGE_QUERYING=3
STAGE_DONE=4

cachefile=/tmp/nerdlog_query_cache

logfile_prev=/var/log/syslog.1
logfile_last=/var/log/syslog

positional_args=()

max_num_lines=100

while [[ $# -gt 0 ]]; do
  case $1 in
    -c|--cache-file)
      cachefile="$2"
      shift # past argument
      shift # past value
      ;;
    --logfile-last)
      logfile_last="$2"
      shift # past argument
      shift # past value
      ;;
    --logfile-prev)
      logfile_prev="$2"
      shift # past argument
      shift # past value
      ;;
    -f|--from)
      from="$2"
      shift # past argument
      shift # past value
      ;;
    -t|--to)
      to="$2"
      shift # past argument
      shift # past value
      ;;
    -u|--lines-until)
      lines_until="$2"
      shift # past argument
      shift # past value
      ;;
    --refresh-index)
      refresh_index="1"
      shift # past argument
      ;;
    -l|--max-num-lines)
      max_num_lines="$2"
      shift # past argument
      shift # past value
      ;;
    -*|--*)
      echo "Unknown option $1" 1>&2
      exit 1
      ;;
    *)
      positional_args+=("$1") # save positional arg
      shift # past argument
      ;;
  esac
done

set -- "${positional_args[@]}" # restore positional parameters

# Just a hack to account for cases when /var/log/syslog.1 doesn't exist:
# create an empty file and pretend that it's an empty log file.
if [ ! -e "$logfile_prev"  ]; then
  logfile_prev="/tmp/nerdlog-empty-file"
  rm -f $logfile_prev
  touch $logfile_prev
fi

logfile_prev_size=$(stat -c%s $logfile_prev)
logfile_last_size=$(stat -c%s $logfile_last)
total_size=$((logfile_prev_size+logfile_last_size))

user_pattern=$1

if [[ "$refresh_index" == "1" ]]; then
  rm -f $cachefile
fi

# NOTE: we only show percentages with 5% increments, to save on traffic and
# other overhead. With all 24 my-nodes, having percentage being printed with
# 1% increments, it generates extra traffic of about 290KB per single query,
# wow. With 5% increments, the overhead is about 70 KB.
awk_func_print_percentage='
function printPercentage(numCur, numTotal) {
  curPercent = int(numCur/numTotal*20);
  if (curPercent != lastPercent) {
    print "p:p:" curPercent*5 >> "/dev/stderr"
    lastPercent = curPercent
  }
}
'

function refresh_cache { # {{{
  local last_linenr=0
  local last_bytenr=0
  local prevlog_bytes=$(get_prevlog_bytenr)

  # Add new entries to cache, if needed

  # NOTE: syslogFieldsToTimestamp parses the traditional systemd timestamp
  # format, like this: "Apr  5 11:07:46". But in the recent versions of
  # rsyslog, it's not the default; that traditional timestamp format can be
  # enabled by adding this line:
  #
  # $ActionFileDefaultTemplate RSYSLOG_TraditionalFileFormat
  #
  # to /etc/rsyslog.conf
  #
  # To use ISO 1806 instead (which is the default in recent rsyslog versions),
  # like "2025-04-05T11:07:46.161001+03:00":
  #
  # $ActionFileDefaultTemplate RSYSLOG_FileFormat
  #
  # But this function (and its usages) need to be updated to support it, and a
  # bunch of other time-filtering logic here. Although it's cool since it
  # includes the year, microseconds, and timezone.
  awk_functions='
function syslogFieldsToTimestamp(monthStr, day, hhmmss) {
  monthByName["Jan"] = "01";
  monthByName["Feb"] = "02";
  monthByName["Mar"] = "03";
  monthByName["Apr"] = "04";
  monthByName["May"] = "05";
  monthByName["Jun"] = "06";
  monthByName["Jul"] = "07";
  monthByName["Aug"] = "08";
  monthByName["Sep"] = "09";
  monthByName["Oct"] = "10";
  monthByName["Nov"] = "11";
  monthByName["Dec"] = "12";

  month = monthByName[monthStr]
  year = 2025
  hour = substr(hhmmss, 1, 2)
  min = substr(hhmmss, 4, 2)

  return mktime(year " " month " " day " " hour " " min " " 0)
}

function nerdlogFieldsToTimestamp(year, month, day, hhmmss) {
  hour = substr(hhmmss, 1, 2)
  min = substr(hhmmss, 4, 2)

  return mktime(year " " month " " day " " hour " " min " " 0)
}

function formatNerdlogTime(timestamp) {
  return strftime("%Y-%m-%d-%H:%M", timestamp)
}

function nerdlogTimestrToTimestamp(timestr) {
  return nerdlogFieldsToTimestamp(substr(timestr, 1, 4), substr(timestr, 6, 2), substr(timestr, 9, 2), substr(timestr, 12, 5));
}

function printIndexLine(outfile, timestr, linenr, bytenr) {
  print "idx\t" timestr "\t" linenr "\t" bytenr >> outfile;
}

function printAllNew(outfile, lastTimestamp, lastTimestr, curTimestamp, curTimestr, linenr, bytenr) {
  if (lastTimestr == "") {
    printIndexLine(outfile, curTimestr, linenr, bytenr);
    return;
  }

  i = 0;
  do {
    nextTimestamp = lastTimestamp + 60
    nextTimestr = formatNerdlogTime(nextTimestamp)

    printIndexLine(outfile, nextTimestr, linenr, bytenr);

    lastTimestamp = nextTimestamp
  } while (nextTimestamp < curTimestamp && i++ < 1000);
}

'$awk_func_print_percentage'
  '
# TODO: ^ newer versions of awk support one more argument for mktime, which is to
#         use UTC. Sadly versions deployed to our machines have older awk, but
#         fortunately they all use UTC as local time, so shouldn't be an issue.
#         Harder to debug locally though.
# TODO: ^ move initialization of monthByName out of logFieldsToTimestamp somehow
# TODO: ^ year needs to be inferred instead of hardcoding 2025
# TODO: ^ if we fail to find the next timestamp and abort on 1000, print an error,
# and then the Go part should see this error and report it to user

# NOTE: this script MUST be executed with the "-b" awk key, which means that
# awk will work in terms of bytes, not characters. We use length($0) there and
# we rely on it being number of bytes.
# TODO: better rewrite this indexing stuff in perl.

  scriptInitFromLastTimestr='
    lastTimestamp = nerdlogTimestrToTimestamp(lastTimestr);
    lastHHMM = substr(lastTimestr, 8, 5);
    last3 = lastHHMM ":00"'

  scriptSetCurTimestr='bytenr_cur = bytenr_next-length($0)-1; curTimestamp = syslogFieldsToTimestamp($1, $2, $3); curTimestr = formatNerdlogTime(curTimestamp)'
  scriptSetLastTimestrEtc='lastTimestr = curTimestr; lastTimestamp = curTimestamp; lastHHMM = curHHMM'

  script1='BEGIN { bytenr_next=1; lastPercent=0 }
{
  bytenr_next += length($0)+1

  if (last3 == $3) {
    next;
  }

  last3 = $3;
  curHHMM = substr($3, 1, 5);
}'

  if [ -s $cachefile ]
  then
    echo "p:stage:$STAGE_INDEX_APPEND:indexing up" 1>&2

    local lastTimestr="$(tail -n 1 $cachefile | cut -f2)"
    local last_linenr="$(tail -n 1 $cachefile | cut -f3)"
    local last_bytenr="$(tail -n 1 $cachefile | cut -f4)"
    local size_to_index=$((total_size-last_bytenr))

    echo debug:hey $lastTimestr 1>&2
    echo debug:hey2 $last_linenr $last_bytenr 1>&2
    echo debug:hey3 $size_to_index 1>&2

    tail -c +$((last_bytenr-prevlog_bytes)) $logfile_last | awk -b "$awk_functions BEGIN { lastTimestr = \"$lastTimestr\"; $scriptInitFromLastTimestr }"'
  '"$script1"'
  ( lastHHMM != curHHMM ) {
    '"$scriptSetCurTimestr"';
    printAllNew("'$cachefile'", lastTimestamp, lastTimestr, curTimestamp, curTimestr, NR+'$(( last_linenr-1 ))', bytenr_cur+'$(( last_bytenr-1 ))');
    printPercentage(bytenr_cur, '$size_to_index');
    '"$scriptSetLastTimestrEtc"'
  }
  ' -
  else
    echo "p:stage:$STAGE_INDEX_FULL:indexing from scratch" 1>&2

    echo "prevlog_modtime	$(stat -c %y $logfile_prev)" > $cachefile

    awk -b "$awk_functions BEGIN { lastHHMM=\"\"; last3=\"\" }"'
  '"$script1"'
  ( lastHHMM != curHHMM ) {
    '"$scriptSetCurTimestr"';
    printAllNew("'$cachefile'", lastTimestamp, lastTimestr, curTimestamp, curTimestr, NR, bytenr_cur);
    printPercentage(bytenr_cur, '$total_size');
    '"$scriptSetLastTimestrEtc"'
  }
  END { print "prevlog_lines\t" NR >> "'$cachefile'" }
  ' $logfile_prev

  # Before we start handling $logfile_last, gotta read the last idx line (which is
  # last-but-one line) and set it for the next script, otherwise there is a gap
  # in index before the first line in the $logfile_last.
  # TODO: make sure that if there are no logs in the $lotfile1, we don't screw up.
    local lastTimestr="$(tail -n 2 $cachefile | head -n 1 | cut -f2)"
    #echo debug:hey3 $lastTimestr 1>&2
    awk -b "$awk_functions BEGIN { lastTimestr = \"$lastTimestr\"; $scriptInitFromLastTimestr }"'
  '"$script1"'
  ( lastHHMM != curHHMM ) {
    '"$scriptSetCurTimestr"';
    bytenr = bytenr_cur+'$prevlog_bytes';
    printAllNew("'$cachefile'", lastTimestamp, lastTimestr, curTimestamp, curTimestr, NR+'$(get_prevlog_lines_from_cache)', bytenr);
    printPercentage(bytenr, '$total_size');
    '"$scriptSetLastTimestrEtc"'
  }
  ' $logfile_last
  fi
} # }}}

# Prints linenumber and bytenumber, space-separated.
# One possible use is:
#   read -r my_linenr my_bytenr <<<$(get_linenr_and_bytenr_from_cache my_timestr)
#
# Now we can use those vars $my_linenr and $my_bytenr
function get_linenr_and_bytenr_from_cache() { # {{{
  awk -F"\t" '$1 == "idx" && $2 == "'$1'" { print $3 " " $4; exit }' $cachefile
} # }}}

function get_prevlog_lines_from_cache() { # {{{
  if ! awk -F"\t" 'BEGIN { found=0 } $1 == "prevlog_lines" { print $2; found = 1; exit } END { if (found == 0) { exit 1 } }' $cachefile ; then
    return 1
  fi
} # }}}

function get_prevlog_modtime_from_cache() { # {{{
  if ! awk -F"\t" 'BEGIN { found=0 } $1 == "prevlog_modtime" { print $2; found = 1; exit } END { if (found == 0) { exit 1 } }' $cachefile ; then
    return 1
  fi
} # }}}

function get_prevlog_bytenr() { # {{{
  du -sb $logfile_prev | awk '{ print $1 }'
} # }}}

if [[ "$from" != "" || "$to" != "" ]]; then
  # Check timestamp in the first line of /tmp/nerdlog_query_cache, and if
  # $logfile_prev's modification time is newer, then delete whole cache
  logfile_prev_stored_modtime="$(get_prevlog_modtime_from_cache)"
  logfile_prev_cur_modtile=$(stat -c %y $logfile_prev)
  if [[ "$logfile_prev_stored_modtime" != "$logfile_prev_cur_modtile" ]]; then
    echo "debug:logfile has changed: stored '$logfile_prev_stored_modtime', actual '$logfile_prev_cur_modtile', deleting it" 1>&2
    rm -f $cachefile
  fi

  if ! get_prevlog_lines_from_cache > /dev/null; then
    echo "debug:broken cache file (no prevlog lines), deleting it" 1>&2
    rm -f $cachefile
  fi

  refresh_and_retry=0

  # First try to find it in cache without refreshing the cache

  if [[ "$from" != "" ]]; then
    read -r from_linenr from_bytenr <<<$(get_linenr_and_bytenr_from_cache $from)
    if [[ "$from_bytenr" == "" ]]; then
      echo "debug:the from isn't found, gonna refresh the cache" 1>&2
      refresh_and_retry=1
    fi
  fi

  if [[ "$to" != "" ]]; then
    read -r to_linenr to_bytenr <<<$(get_linenr_and_bytenr_from_cache $to)
    if [[ "$to_bytenr" == "" ]]; then
      echo "debug:the to isn't found, gonna refresh the cache" 1>&2
      refresh_and_retry=1
    fi
  fi

  if [[ "$refresh_and_retry" == 1 ]]; then
    refresh_cache

    if [[ "$from" != "" ]]; then
      read -r from_linenr from_bytenr <<<$(get_linenr_and_bytenr_from_cache $from)
      if [[ "$from_bytenr" == "" ]]; then
        echo "debug:the from isn't found, will use the beginning" 1>&2
      fi
    fi

    if [[ "$to" != "" ]]; then
      read -r to_linenr to_bytenr <<<$(get_linenr_and_bytenr_from_cache $to)
      if [[ "$to_bytenr" == "" ]]; then
        echo "debug:the to isn't found, will use the end" 1>&2
      fi
    fi

  fi
else
  if ! [ -s $cachefile ]; then
    echo "debug:neither --from or --to are given, but index doesn't exist at all, gonna rebuild" 1>&2
    refresh_cache
  fi
fi


echo "debug:from $from_linenr ($from_bytenr) to $to_linenr ($to_bytenr)" 1>&2

echo "p:stage:$STAGE_QUERYING:querying logs" 1>&2

prevlog_lines=$(get_prevlog_lines_from_cache)
prevlog_bytes=$(get_prevlog_bytenr)

from_linenr_int=$from_linenr
if [[ "$from_linenr" == "" ]]; then
  from_linenr_int=1
fi

awk_pattern=''
if [[ "$user_pattern" != "" ]]; then
  awk_pattern="!($user_pattern) {next}"
fi

lines_until_check=''
if [[ "$lines_until" != "" ]]; then
  lines_until_check="if (NR >= $((lines_until-from_linenr_int+1))) { next; }"
fi

num_bytes_to_scan=0
if [[ "$from_bytenr" == "" && "$to_bytenr" == "" ]]; then
  # Getting _all_ available logs
  num_bytes_to_scan=$total_size
  echo debug:hey1 $num_bytes_to_scan 2>&1
elif [[ "$from_bytenr" != "" && "$to_bytenr" == "" ]]; then
  # Getting logs from some point in time to the very end (most frequent case)
  num_bytes_to_scan=$((total_size-from_bytenr))
  echo debug:hey2 "|$num_bytes_to_scan,$total_size,$from_bytenr|" 2>&1
elif [[ "$from_bytenr" == "" && "$to_bytenr" != "" ]]; then
  # Getting logs from the beginning until some point in time
  num_bytes_to_scan=$((to_bytenr))
  echo debug:hey3 $num_bytes_to_scan 2>&1
else
  # Getting logs between two points T1 and T2
  num_bytes_to_scan=$((to_bytenr-from_bytenr))
  echo debug:hey4 $num_bytes_to_scan 2>&1
fi

# NOTE: this script MUST be executed with the "-b" awk key, which means that
# awk will work in terms of bytes, not characters. We use length($0) there and
# we rely on it being number of bytes. That said, it's only used for percentage
# calculation, which is a non-essential.
#
# Also btw, this percentage calculation slows the whole query by about 10%,
# which isn't ideal. TODO: maybe instead of doing the division on every line,
# we can only do the division when the percentage changes, so we calculate the
# next point when it'd change, and going forward we just compare it with a simple "<".
awk_script='
'$awk_func_print_percentage'

BEGIN { bytenr=1; curline=0; maxlines='$max_num_lines'; lastPercent=0 }
{ bytenr += length($0)+1 }
NR % 100 == 0 {
  printPercentage(bytenr, '$num_bytes_to_scan')
}
'$awk_pattern'
{
  stats[$1 $2 "-" substr($3,1,5)]++;

  '$lines_until_check'

  lastlines[curline] = $0;
  lastNRs[curline] = NR;
  curline++
  if (curline >= maxlines) {
    curline = 0;
  }

  next;
}

END {
  print "logfile:'$logfile_prev':0";
  print "logfile:'$logfile_last':'$prevlog_lines'";

  for (x in stats) {
    print "s:" x "," stats[x]
  }

  for (i = 0; i < maxlines; i++) {
    ln = curline + i;
    if (ln >= maxlines) {
      ln -= maxlines;
    }

    if (!lastlines[ln]) {
      continue;
    }

    curNR = lastNRs[ln] + '$from_linenr_int' - 1;

    print "m:" curNR ":" lastlines[ln];
  }
}
'

# NOTE: there are multiple ways to tail a file, and performance differs greatly:
# Log file has 21789347 lines:
#
#ubuntu@dummy-node-01:~$ time cat /var/log/syslog.1 | tail -n +16789340 > /dev/null

#real    0m4.523s
#user    0m0.869s
#sys     0m6.915s
#ubuntu@dummy-node-01:~$ time tail -n +16789340 /var/log/syslog.1 > /dev/null

#real    0m2.184s
#user    0m0.660s
#sys     0m1.524s
#ubuntu@dummy-node-01:~$ time tail -n 5000000 /var/log/syslog.1 > /dev/null

#real    0m1.260s
#user    0m0.412s
#sys     0m0.848s

# So it's best to tail file directly (without cat) and also whenever possible
# do the "-n N", not "-n +N" (but for the latest logfile, which is constantly
# appended to, we have to use the "-n +N")

# Generate commands to get all the logs as per requested timerange.
declare -a cmds
if [[ "$from_bytenr" != "" && $(( from_bytenr > prevlog_bytes )) == 1 ]]; then
  # Only $logfile_last is used.
  from_bytenr=$(( from_bytenr - prevlog_bytes ))
  if [[ "$to_bytenr" != "" ]]; then
    to_bytenr=$(( to_bytenr - prevlog_bytes ))
    echo "debug:Getting logs from offset $from_bytenr, only $((to_bytenr - from_bytenr)) bytes, all in the latest $logfile_last" 1>&2
    cmds+=("tail -c +$from_bytenr $logfile_last | head -c $((to_bytenr - from_bytenr))")
  else
    # Most common case
    echo "debug:Getting logs from offset $from_bytenr until the end of latest $logfile_last." 1>&2
    cmds+=("tail -c +$from_bytenr $logfile_last")
  fi
elif [[ "$to_bytenr" != "" && $(( to_bytenr <= prevlog_bytes )) == 1 ]]; then
  # Only $logfile_prev is used.
  if [[ "$from_bytenr" != "" ]]; then
    echo "debug:Getting logs from offset $from_bytenr, only $((to_bytenr - from_bytenr)) bytes, all in the prev $logfile_prev" 1>&2
    cmds+=("tail -c +$from_bytenr $logfile_prev | head -c $((to_bytenr - from_bytenr))")
  else
    echo "debug:Getting logs from the very beginning to offset $(( to_bytenr - 1 )), all in the prev $logfile_prev." 1>&2
    cmds+=("head -c $(( to_bytenr - 1)) $logfile_prev")
  fi
else
  # Both log files are used
  if [[ "$from_bytenr" != "" ]]; then
    info="Getting logs from offset $from_bytenr in prev $logfile_prev"
    cmds+=("tail -c +$from_bytenr $logfile_prev")
  else
    info="Getting logs from the very beginning in prev $logfile_prev"
    cmds+=("cat $logfile_prev")
  fi

  if [[ "$to_bytenr" != "" ]]; then
    info="$info to offset $(( to_bytenr - prevlog_bytes - 1 )) in latest $logfile_last"
    cmds+=("head -c $(( to_bytenr - prevlog_bytes - 1 )) $logfile_last")
  else
    info="$info until the end of latest $logfile_last"
    cmds+=("cat $logfile_last")
  fi

  echo "debug:$info" 1>&2
fi

# Now execute all those commands, and feed those logs to the awk script
# which will analyze them and produce the final output.
for cmd in "${cmds[@]}"; do eval $cmd; done | awk -b "$awk_script" -

echo "p:stage:$STAGE_DONE:done" 1>&2
