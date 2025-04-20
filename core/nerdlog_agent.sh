#/bin/bash

trap 'echo "exit_code:$?"' EXIT

# Arguments:
#
# --from, --to: time in the format "2006-01-02-15:04".

# Those numbers are supposed to go up as the query progresses; the Go app
# will then be able to tell which node is the slowest and show info for it.
STAGE_INDEX_FULL=1
STAGE_INDEX_APPEND=2
STAGE_QUERYING=3
STAGE_DONE=4

cachefile=/tmp/nerdlog_agent_cache

logfile_prev=auto
logfile_last=auto

positional_args=()

max_num_lines=100

awktime_month='monthByName[substr($0, 1, 3)]'
awktime_year='yearByMonth[month]'
awktime_day='(substr($0, 5, 1) == " ") ? "0" substr($0, 6, 1) : substr($0, 5, 2)'
awktime_hhmm='substr($0, 8, 5)'
awktime_minute_key='substr($0, 1, 12)'
# TODO: double check that if any of these is provided manually in a flag,
# then all of them are provided manually.

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

    --awktime-month)
      awktime_month="$2"
      shift # past argument
      shift # past value
      ;;
    --awktime-year)
      awktime_year="$2"
      shift # past argument
      shift # past value
      ;;
    --awktime-day)
      awktime_day="$2"
      shift # past argument
      shift # past value
      ;;
    --awktime-hhmm)
      awktime_hhmm="$2"
      shift # past argument
      shift # past value
      ;;
    --awktime-minute-key)
      awktime_minute_key="$2"
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

# Either use the provided current year and month (for tests), or get the actual ones.
if [[ "$CUR_YEAR" == "" ]]; then
  CUR_YEAR="$(date +'%Y')"
fi

if [[ "$CUR_MONTH" == "" ]]; then
  CUR_MONTH="$(date +'%m')"
fi

if [[ "$logfile_last" == "auto" ]]; then
  if [ -e /var/log/messages ]; then
    logfile_last=/var/log/messages
  elif [ -e /var/log/syslog ]; then
    logfile_last=/var/log/syslog
  else
    echo "error:failed to autodetect log file" 1>&2
    exit 1
  fi
fi

if [[ "$logfile_prev" == "auto" ]]; then
  # For now just blindly append ".1" to the first logfile; if it doesn't actually
  # exist, we'll handle this case right below.
  logfile_prev="${logfile_last}.1"
fi

# A simple hack to account for cases when /var/log/syslog.1 doesn't exist:
# create an empty file and pretend that it's an empty log file.
if [ ! -e "$logfile_prev"  ]; then
  echo "debug:prev logfile $logfile_prev doesn't exist, using a dummy empty file /tmp/nerdlog-empty-file" 1>&2
  logfile_prev="/tmp/nerdlog-empty-file"
  rm -f $logfile_prev || exit 1
  touch $logfile_prev || exit 1

  # For stable output in tests, also update the creation/modification time of
  # that file to be the same as the first log file. It's not portable though
  # (not gonna work on BSD), but it's non-essential functionality, so we just
  # ignore any errors here and do nothing then.
  ctime=$(stat -c %W $logfile_last)
  if [[ $? == 0 ]]; then
    touch -d "@$ctime" $logfile_prev
  fi
fi

command="$1"
if [[ "${command}" == "" ]]; then
  echo "error:command is required" 1>&2
  exit 1
fi

case "${command}" in
  query)
    shift
    # Will be handled below.
    ;;

  logstream_info)
    # TODO: support the case where timedatectl is not available
    host_timezone="$(timedatectl show --property=Timezone --value)" || exit 1
    echo "host_timezone:$host_timezone"

    if [ ! -e ${logfile_last} ]; then
      echo "error:${logfile_last} does not exist"
      exit 1
    fi

    if [ ! -r ${logfile_last} ]; then
      echo "error:${logfile_last} exists but is not readable, check your permissions"
      exit 1
    fi

    if [ ! -e ${logfile_prev} ]; then
      echo "error:${logfile_prev} does not exist"
      exit 1
    fi

    if [ ! -r ${logfile_prev} ]; then
      echo "error:${logfile_prev} exists but is not readable, check your permissions"
      exit 1
    fi

    # Print a bunch of example log lines, so that the client can autodetect the
    # format.
    if [ -s ${logfile_last} ]; then
      last_line="$(tail -n 1 ${logfile_last})" || exit 1
      first_line="$(head -n 1 ${logfile_last})" || exit 1
      echo "example_log_line:$last_line"
      echo "example_log_line:$first_line"
    fi
    if [ -s ${logfile_prev} ]; then
      last_line="$(tail -n 1 ${logfile_prev})" || exit 1
      first_line="$(head -n 1 ${logfile_prev})" || exit 1
      echo "example_log_line:$last_line"
      echo "example_log_line:$first_line"
    fi

    exit 0
    ;;

  *)
    echo "error:invalid command ${command}" 1>&2
    exit 1
esac

# What follows is the handler for the "query" command.

user_pattern=$1

logfile_prev_size=$(stat -c%s $logfile_prev) || exit 1
logfile_last_size=$(stat -c%s $logfile_last) || exit 1
total_size=$((logfile_prev_size+logfile_last_size)) || exit 1

if [[ "$refresh_index" == "1" ]]; then
  rm -f $cachefile || exit 1
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

  awk_vars='
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

    curYear = '${CUR_YEAR}';
    curMonth = '${CUR_MONTH}';

    yearByMonth["01"] = inferYear(1, curYear, curMonth) "";
    yearByMonth["02"] = inferYear(2, curYear, curMonth) "";
    yearByMonth["03"] = inferYear(3, curYear, curMonth) "";
    yearByMonth["04"] = inferYear(4, curYear, curMonth) "";
    yearByMonth["05"] = inferYear(5, curYear, curMonth) "";
    yearByMonth["06"] = inferYear(6, curYear, curMonth) "";
    yearByMonth["07"] = inferYear(7, curYear, curMonth) "";
    yearByMonth["08"] = inferYear(8, curYear, curMonth) "";
    yearByMonth["09"] = inferYear(9, curYear, curMonth) "";
    yearByMonth["10"] = inferYear(10, curYear, curMonth) "";
    yearByMonth["11"] = inferYear(11, curYear, curMonth) "";
    yearByMonth["12"] = inferYear(12, curYear, curMonth) "";
  '

  # Add new entries to cache, if needed

  # NOTE: syslogFieldsToIndexTimestr parses the traditional systemd timestamp
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
function inferYear(logMonth, curYear, curMonth) {
  delta = logMonth - curMonth

  if (delta <= -11)       # log month is Jan, current is Dec -> next year
    return curYear + 1
  else if (delta >= 8)    # log month is Sep-Dec, current is Jan -> previous year
    return curYear - 1
  else
    return curYear
}

function printIndexLine(outfile, timestr, linenr, bytenr) {
  print "idx\t" timestr "\t" linenr "\t" bytenr >> outfile;
}

'$awk_func_print_percentage'
  '
# NOTE: this script MUST be executed with the "-b" awk key, which means that
# awk will work in terms of bytes, not characters. We use length($0) there and
# we rely on it being number of bytes.
# TODO: better rewrite this indexing stuff in perl.

  scriptInitFromLastTimestr='
    lastHHMM = substr(lastTimestr, 8, 5);
    last3 = lastHHMM ":00"'

  scriptSetCurTimestr='
    bytenr_cur = bytenr_next - length($0) - 1;

    month = '"$awktime_month"';
    year = '"$awktime_year"';
    day = '"$awktime_day"';
    hhmm = '"$awktime_hhmm"';

    curTimestr = year "-" month "-" day "-" hhmm;
    if (curTimestr < lastTimestr) {
      print "error:timestamp decreased from " lastTimestr " to " curTimestr ", might be using inconsistent timestamp formats" > "/dev/stderr"
      exit 1
    }
  '
  scriptSetLastTimestrEtc='
    lastTimestr = curTimestr;
    lastHHMM = curHHMM;
  '

  script1='BEGIN { bytenr_next=1; lastPercent=0 }
{
  bytenr_next += length($0)+1
  curHHMM = '"$awktime_hhmm"';
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

    tail -c +$((last_bytenr-prevlog_bytes)) $logfile_last | awk -b "$awk_functions
  BEGIN {
    $awk_vars
    lastTimestr = \"$lastTimestr\"; $scriptInitFromLastTimestr
  }"'
  '"$script1"'
  ( lastHHMM != curHHMM ) {
    '"$scriptSetCurTimestr"';
    printIndexLine("'$cachefile'", curTimestr, NR+'$(( last_linenr-1 ))', bytenr_cur+'$(( last_bytenr-1 ))');
    printPercentage(bytenr_cur, '$size_to_index');
    '"$scriptSetLastTimestrEtc"'
  }
  ' -
    if [[ "$?" != 0 ]]; then
      echo "debug:failed to index up, removing index file" 1>&2
      rm $cachefile
      exit 1
    fi
  else
    echo "p:stage:$STAGE_INDEX_FULL:indexing from scratch" 1>&2

    echo "prevlog_modtime	$(stat -c %y $logfile_prev)" > $cachefile

    awk -b "$awk_functions BEGIN { $awk_vars lastHHMM=\"\"; last3=\"\" }"'
  '"$script1"'
  ( lastHHMM != curHHMM ) {
    '"$scriptSetCurTimestr"';
    printIndexLine("'$cachefile'", curTimestr, NR, bytenr_cur);
    printPercentage(bytenr_cur, '$total_size');
    '"$scriptSetLastTimestrEtc"'
  }
  END { print "prevlog_lines\t" NR >> "'$cachefile'" }
  ' $logfile_prev
    if [[ "$?" != 0 ]]; then
      echo "debug:failed to index from scratch $logfile_prev, removing index file" 1>&2
      rm $cachefile
      exit 1
    fi

  # Before we start handling $logfile_last, gotta read the last idx line (which is
  # last-but-one line) and set it for the next script, otherwise there is a gap
  # in index before the first line in the $logfile_last.
  # TODO: make sure that if there are no logs in the $lotfile1, we don't screw up.
    local lastTimestr=""
    local lastTimestrLine="$(tail -n 2 $cachefile | head -n 1)"
    if [[ "$lastTimestrLine" =~ ^idx$'\t' ]]; then
      lastTimestr="$(echo "$lastTimestrLine" | cut -f2)"
    fi
    #echo debug:hey3 $lastTimestr 1>&2
    awk -b "$awk_functions BEGIN { $awk_vars lastTimestr = \"$lastTimestr\"; $scriptInitFromLastTimestr }"'
  '"$script1"'
  ( lastHHMM != curHHMM ) {
    '"$scriptSetCurTimestr"';
    bytenr = bytenr_cur+'$prevlog_bytes';
    printIndexLine("'$cachefile'", curTimestr, NR+'$(get_prevlog_lines_from_cache)', bytenr);
    printPercentage(bytenr, '$total_size');
    '"$scriptSetLastTimestrEtc"'
  }
  ' $logfile_last
    if [[ "$?" != 0 ]]; then
      echo "debug:failed to index from scratch $logfile_last, removing index file" 1>&2
      rm $cachefile
      exit 1
    fi
  fi
} # }}}

# Performs index lookup by a timestr like "2006-01-02-15:04" (typically given
# as --from or --to, and it's also stored in the index in the same form).
#
# Prints result: one of "found", "before" or "after"; and if the result
# is "found", then also prints linenumber and bytenumber, space-separated.
# "before" means the given timestr is earlier than the earliest log we have,
# and "after" obviously means that it's later than the latest log we have.
#
# One possible use is:
#   read -r my_result my_linenr my_bytenr <<<$(get_linenr_and_bytenr_from_cache my_timestr)
#
# Now we can use those vars $my_result, $my_linenr and $my_bytenr
function get_linenr_and_bytenr_from_cache() { # {{{
  awk -F"\t" '
    BEGIN { isFirstIdx = 1; }
    $1 == "idx" {
      if ("'$1'" == $2) {
        print "found " $3 " " $4;
        exit
      } else if ("'$1'" < $2) {
        if (isFirstIdx) {
          print "before";
        } else {
          print "found " $3 " " $4;
        }
        exit
      } else {
        isFirstIdx = 0;
      }
    }
    END { print "after"; exit }
  ' $cachefile
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

is_outside_of_range=0
if [[ "$from" != "" || "$to" != "" ]]; then
  # If indexfile exists, check if it's valid and relevant; if not, delete it.
  if [ -e "$cachefile" ]; then
    # Check timestamp in the first line of /tmp/nerdlog_agent_cache, and if
    # $logfile_prev's modification time is newer, then delete whole cache
    logfile_prev_stored_modtime="$(get_prevlog_modtime_from_cache)"
    logfile_prev_cur_modtile=$(stat -c %y $logfile_prev)
    if [[ "$logfile_prev_stored_modtime" != "$logfile_prev_cur_modtile" ]]; then
      echo "debug:logfile has changed: stored '$logfile_prev_stored_modtime', actual '$logfile_prev_cur_modtile', deleting index file" 1>&2
      rm -f $cachefile || exit 1
    fi

    if ! get_prevlog_lines_from_cache > /dev/null; then
      echo "debug:broken cache file (no prevlog lines), deleting it" 1>&2
      rm -f $cachefile || exit 1
    fi
  fi

  refresh_and_retry=0

  # First try to find it in cache without refreshing the cache

  if [ -s "$cachefile" ]; then
    if [[ "$from" != "" ]]; then
        read -r from_result from_linenr from_bytenr <<<$(get_linenr_and_bytenr_from_cache "$from") || exit 1
        if [[ "$from_result" != "found" ]]; then
          echo "debug:the from ${from} isn't found, gonna refresh the cache" 1>&2
          refresh_and_retry=1
        fi
    fi

    if [[ "$to" != "" ]]; then
      read -r to_result to_linenr to_bytenr <<<$(get_linenr_and_bytenr_from_cache "$to") || exit 1
      if [[ "$to_result" != "found" ]]; then
        echo "debug:the to ${to} isn't found, gonna refresh the cache" 1>&2
        refresh_and_retry=1
      fi
    fi
  else
    echo "debug:index file doesn't exist or is empty, gonna refresh it" 1>&2
    refresh_and_retry=1
  fi

  if [[ "$refresh_and_retry" == 1 ]]; then
    refresh_cache || exit 1

    if [[ "$from" != "" ]]; then
      read -r from_result from_linenr from_bytenr <<<$(get_linenr_and_bytenr_from_cache "$from") || exit 1

      if [[ "$from_result" == "before" ]]; then
        echo "debug:the from ${from} isn't found, will use the beginning" 1>&2
      elif [[ "$from_result" == "found" ]]; then
        echo "debug:the from ${from} is found: $from_linenr ($from_bytenr)" 1>&2
        if [[ "$from_bytenr" == "" || "$from_linenr" == "" ]]; then
          echo "error:from_result is found but from_bytenr and/or from_linenr is empty" 1>&2
          exit 1
        fi
      elif [[ "$from_result" == "after" ]]; then
        echo "debug:the from ${from} is after the latest log we have, will return nothing" 1>&2
        is_outside_of_range=1
      else
        echo "error:invalid from_result: $from_result" 1>&2
        exit 1
      fi
    fi

    if [[ "$to" != "" ]]; then
      read -r to_result to_linenr to_bytenr <<<$(get_linenr_and_bytenr_from_cache "$to") || exit 1

      if [[ "$to_result" == "after" ]]; then
        echo "debug:the to ${to} isn't found, will use the end" 1>&2
      elif [[ "$to_result" == "found" ]]; then
        echo "debug:the to ${to} is found: $to_linenr ($to_bytenr)" 1>&2
        if [[ "$to_bytenr" == "" || "$to_linenr" == "" ]]; then
          echo "error:to_result is found but to_bytenr and/or to_linenr is empty" 1>&2
          exit 1
        fi
      elif [[ "$to_result" == "before" ]]; then
        echo "debug:the to ${to} is before the first log we have, will return nothing" 1>&2
        is_outside_of_range=1
      else
        echo "error:invalid to_result: $to_result" 1>&2
        exit 1
      fi
    fi

  fi
else
  if ! [ -s $cachefile ]; then
    echo "debug:neither --from or --to are given, but index doesn't exist at all, gonna rebuild" 1>&2
    refresh_cache || exit 1
  fi
fi

if [[ $is_outside_of_range == 1 ]]; then
  echo "p:stage:$STAGE_DONE:done" 1>&2
  exit 0
fi

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
  stats['"$awktime_minute_key"']++;

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
for cmd in "${cmds[@]}"; do eval $cmd || exit 1; done | awk -b "$awk_script" -

if ! [[ ${PIPESTATUS[@]} =~ ^(0[[:space:]]*)+$ ]]; then
  exit 1
fi

echo "p:stage:$STAGE_DONE:done" 1>&2
