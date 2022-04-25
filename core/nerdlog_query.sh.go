package core

// TODO: convert it to an embedded file
var nerdlogQuerySh = `#/bin/bash

# Arguments:
#
# --from, --to: time in the format Jan-02-15:04. NOTE it's important to keep
#               the leading zero!

cachefile=/tmp/nerdlog_query_cache

logfile1=/var/log/syslog.1
logfile2=/var/log/syslog

positional_args=()

max_num_lines=100

while [[ $# -gt 0 ]]; do
  case $1 in
    -c|--cache-file)
      cachefile="$2"
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

user_pattern=$1

function refresh_cache { # {{{
  local lastnr=0
  local awknrplus="NR"

  # Add new entries to cache, if needed

  awk_functions='
function logFieldsToTimestamp(monthStr, day, hhmm) {
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
  year = 2022
  hour = substr(hhmm, 1, 2)
  min = substr(hhmm, 4, 2)

  return mktime(year " " month " " day " " hour " " min " " 0)
}

function formatNerdlogTime(timestamp) {
  return strftime("%b-%d-%H:%M", timestamp, 1)
}

function nerdlogTimestrToTImestamp(timestr) {
  return logFieldsToTimestamp(substr(timestr, 1, 3), substr(timestr, 5, 2), substr(timestr, 8, 5));
}

function printIndexLine(timestr, linenr) {
  print "idx\t" timestr "\t" linenr;
}

function printAllNew(lastTimestamp, lastTimestr, curTimestamp, curTimestr, linenr) {
  if (lastTimestr == "") {
    printIndexLine(curTimestr, linenr);
    return;
  }

  i = 0;
  do {
    nextTimestamp = lastTimestamp + 60
    nextTimestr = formatNerdlogTime(nextTimestamp)

    printIndexLine(nextTimestr, linenr);

    lastTimestamp = nextTimestamp
  } while (nextTimestamp < curTimestamp && i++ < 1000);
}

  '
# TODO: ^ newer versions of awk support one more argument for mktime, which is to
#         use UTC. Sadly versions deployed to our machines have older awk, but
#         fortunately they all use UTC as local time, so shouldn't be an issue.
#         Harder to debug locally though.
# TODO: ^ move initialization of monthByName out of logFieldsToTimestamp somehow
# TODO: ^ year needs to be inferred instead of hardcoding 2022
# TODO: ^ if we fail to find the next timestamp and abort on 1000, print an error,
# and then the Go part should see this error and report it to user

  script1='{
  curTimestamp = logFieldsToTimestamp($1, $2, $3)
  curTimestr = formatNerdlogTime(curTimestamp)
}'

  if [ -s $cachefile ]; then
    echo "caching new line numbers" 1>&2

    local typ="$(tail -n 1 $cachefile | cut -f1)"
    local lastTimestr="$(tail -n 1 $cachefile | cut -f2)"
    local lastnr="$(tail -n 1 $cachefile | cut -f3)"
    local awknrplus="NR+$(( lastnr-1 ))"

    echo hey $lastTimestr 1>&2
    echo hey2 $lastnr 1>&2
    #lastnr=$(( lastnr-1 ))

    # TODO: as one more optimization, we can store the size of the logfile1 in
    # the cache, so here we get this file size and below we don't cat it.
    local logfile1_numlines=0

    cat $logfile1 $logfile2 | tail -n +$((lastnr-logfile1_numlines)) | awk "$awk_functions BEGIN { lastTimestr = \"$lastTimestr\"; lastTimestamp = nerdlogTimestrToTImestamp(lastTimestr) }"'
  '"$script1"'
  ( lastTimestr != curTimestr ) { printAllNew(lastTimestamp, lastTimestr, curTimestamp, curTimestr, NR+'$(( lastnr-1 ))'); lastTimestr = curTimestr; lastTimestamp = curTimestamp; }
  ' - >> $cachefile
  else
    echo "caching all line numbers" 1>&2

    echo "prevlog_modtime	$(stat -c %y $logfile1)" > $cachefile

    cat $logfile1 | awk "$awk_functions"'
  '"$script1"'
  ( lastTimestr != curTimestr ) { printAllNew(lastTimestamp, lastTimestr, curTimestamp, curTimestr, NR); lastTimestr = curTimestr; lastTimestamp = curTimestamp; }
  END { print "prevlog_lines\t" NR }
  ' - >> $cachefile

  # Before we start handling $logfile2, gotta read the last idx line (which is
  # last-but-one line) and set it for the next script, otherwise there is a gap
  # in index before the first line in the $logfile2.
  # TODO: make sure that if there are no logs in the $lotfile1, we don't screw up.
    local lastTimestr="$(tail -n 2 $cachefile | head -n 1 | cut -f2)"
    #echo hey3 $lastTimestr 1>&2
    cat $logfile2 | awk "$awk_functions BEGIN { lastTimestr = \"$lastTimestr\"; lastTimestamp = nerdlogTimestrToTImestamp(lastTimestr) }"'
  '"$script1"'
  ( lastTimestr != curTimestr ) { printAllNew(lastTimestamp, lastTimestr, curTimestamp, curTimestr, NR+'$(get_prevlog_lines_from_cache)'); lastTimestr = curTimestr;  lastTimestamp = curTimestamp; }
  ' - >> $cachefile
  fi
} # }}}

function get_from_cache() { # {{{
  awk -F"\t" '$1 == "idx" && $2 == "'$1'" { print $3; exit }' $cachefile
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

if [[ "$from" != "" || "$to" != "" ]]; then
  # Check timestamp in the first line of /tmp/nerdlog_query_cache, and if
  # $logfile1's modification time is newer, then delete whole cache
  logfile1_stored_modtime="$(get_prevlog_modtime_from_cache)"
  logfile1_cur_modtile=$(stat -c %y $logfile1)
  if [[ "$logfile1_stored_modtime" != "$logfile1_cur_modtile" ]]; then
    echo "logfile has changed: stored '$logfile1_stored_modtime', actual '$logfile1_cur_modtile', deleting it" 1>&2
    rm -f $cachefile
  fi

  if ! get_prevlog_lines_from_cache > /dev/null; then
    echo "broken cache file (no prevlog lines), deleting it" 1>&2
    rm -f $cachefile
  fi

  refresh_and_retry=0

  # First try to find it in cache without refreshing the cache

  if [[ "$from" != "" ]]; then
    from_nr=$(get_from_cache $from)
    if [[ "$from_nr" == "" ]]; then
      echo "the from isn't found, gonna refresh the cache" 1>&2
      refresh_and_retry=1
    fi
  fi

  if [[ "$to" != "" ]]; then
    to_nr=$(get_from_cache $to)
    if [[ "$to_nr" == "" ]]; then
      echo "the to isn't found, gonna refresh the cache" 1>&2
      refresh_and_retry=1
    fi
  fi

  if [[ "$refresh_and_retry" == 1 ]]; then
    refresh_cache

    if [[ "$from" != "" ]]; then
      from_nr=$(get_from_cache $from)
      if [[ "$from_nr" == "" ]]; then
        echo "the from isn't found, will use the beginning" 1>&2
      fi
    fi

    if [[ "$to" != "" ]]; then
      to_nr=$(get_from_cache $to)
      if [[ "$to_nr" == "" ]]; then
        echo "the to isn't found, will use the end" 1>&2
      fi
    fi

  fi
fi

echo "from $from_nr to $to_nr" 1>&2

echo "scanning logs" 1>&2

prevlog_lines=$(get_prevlog_lines_from_cache)

from_nr_int=$from_nr
if [[ "$from_nr" == "" ]]; then
  from_nr_int=0
fi

awk_pattern=''
if [[ "$user_pattern" != "" ]]; then
  awk_pattern="!($user_pattern) {next}"
fi

lines_until_check=''
if [[ "$lines_until" != "" ]]; then
  lines_until_check="if (NR >= $((lines_until-from_nr_int+1))) { next; }"
fi

awk_script='
BEGIN { curline=0; maxlines='$max_num_lines' }
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
  print "logfile:'$logfile1':0";
  print "logfile:'$logfile2':'$prevlog_lines'";

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

    curNR = lastNRs[ln] + '$from_nr_int' - 1;

    print "m:" curNR ":" lastlines[ln];
  }
}
'

# NOTE: there are multiple ways to tail a file, and performance differs greatly:
# Log file has 21789347 lines:
#
#ubuntu@my-host-1-watchdog-01:~$ time cat /var/log/syslog.1 | tail -n +16789340 > /dev/null

#real    0m4.523s
#user    0m0.869s
#sys     0m6.915s
#ubuntu@my-host-1-watchdog-01:~$ time tail -n +16789340 /var/log/syslog.1 > /dev/null

#real    0m2.184s
#user    0m0.660s
#sys     0m1.524s
#ubuntu@my-host-1-watchdog-01:~$ time tail -n 5000000 /var/log/syslog.1 > /dev/null

#real    0m1.260s
#user    0m0.412s
#sys     0m0.848s

# So it's best to tail file directly (without cat) and also whenever possible
# do the "-n N", not "-n +N" (but for the latest logfile, which is constantly
# appended to, we have to use the "-n +N")

# Generate commands to get all the logs as per requested timerange.
declare -a cmds
if [[ "$from_nr" != "" && $(( from_nr > prevlog_lines )) == 1 ]]; then
  # Only $logfile2 is used.
  from_nr=$(( from_nr - prevlog_lines ))
  if [[ "$to_nr" != "" ]]; then
    echo "Getting logs from line $from_nr to line $((to_nr-1)) inclusive, all in the $logfile2" 1>&2
    to_nr=$(( to_nr - prevlog_lines ))
    cmds+=("tail -n +$from_nr $logfile2 | head -n $((to_nr - from_nr))")
  else
    # Most common case
    echo "Getting logs from line $from_nr until the end of $logfile2." 1>&2
    cmds+=("tail -n +$from_nr $logfile2")
  fi
elif [[ "$to_nr" != "" && $(( to_nr <= prevlog_lines )) == 1 ]]; then
  # Only $logfile1 is used.
  if [[ "$from_nr" != "" ]]; then
    echo "Getting logs from line $from_nr to line $((to_nr-1)), all in the $logfile1" 1>&2
    # TODO: see similar TODO below for the tail -n
    cmds+=("tail -n $(( prevlog_lines - from_nr + 1 )) $logfile1 | head -n $((to_nr - from_nr))")
  else
    echo "Getting logs from the very beginning to line $(( to_nr - 1 )) inclusive, all in the $logfile1." 1>&2
    cmds+=("head -n $(( to_nr - 1)) $logfile1")
  fi
else
  # Both log files are used
  if [[ "$from_nr" != "" ]]; then
    info="Getting logs from line $from_nr in $logfile1"
    # TODO: for now we just assume that from_nr is closer to the end of
    # $logfile1, and so we use the usual tail (specifying number of lines in
    # the end), but it's better to check if from_nr is actually closer to the
    # beginning, and then use the tail -n +$(( from_nr ))
    cmds+=("tail -n $(( prevlog_lines - from_nr + 1 )) $logfile1")
  else
    info="Getting logs from the very beginning in $logfile1"
    cmds+=("cat $logfile1")
  fi

  if [[ "$to_nr" != "" ]]; then
    info="$info to line $(( to_nr - prevlog_lines - 1 )) inclusive in $logfile2"
    cmds+=("head -n $(( to_nr - prevlog_lines - 1 )) $logfile2")
  else
    info="$info until the end of $logfile2"
    cmds+=("cat $logfile2")
  fi

  echo "$info" 1>&2
fi

# Now execute all those commands, and feed those logs to the awk script
# which will analyze them and produce the final output.
for cmd in "${cmds[@]}"; do eval $cmd; done | awk "$awk_script" -
`
