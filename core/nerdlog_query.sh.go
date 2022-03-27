package core

import "strconv"

// TODO: convert it to an embedded file
var nerdlogQuerySh = `#/bin/bash

cachefile=/tmp/nerdlog_query_cache

logfile1=/var/log/syslog.1
logfile2=/var/log/syslog

positional_args=()

while [[ $# -gt 0 ]]; do
  case $1 in
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
  if [ -s $cachefile ]; then
    echo "caching new line numbers" 1>&2

    local typ="$(tail -n 1 $cachefile | cut -f1)"
    local lastts="$(tail -n 1 $cachefile | cut -f2)"
    local lastnr="$(tail -n 1 $cachefile | cut -f3)"
    local awknrplus="NR+$(( lastnr-1 ))"

    echo hey $lastts 1>&2
    echo hey2 $lastnr 1>&2
    #lastnr=$(( lastnr-1 ))

    # TODO: as one more optimization, we can store the size of the logfile1 in
    # the cache, so here we get this file size and below we don't cat it.
    local logfile1_numlines=0

    cat $logfile1 $logfile2 | tail -n +$((lastnr-logfile1_numlines)) | awk "BEGIN { lastts = \"$lastts\" }"'
  { curts = $1 "-" $2 "-" substr($3, 1, 5) }
  ( lastts != curts ) { print "idx\t" curts "\t" NR+'$(( lastnr-1 ))'; lastts = curts }
  ' - >> $cachefile
  else
    echo "caching all line numbers" 1>&2

    echo "prevlog_modtime	$(stat -c %y $logfile1)" > $cachefile

    cat $logfile1 | awk '
  { curts = $1 "-" $2 "-" substr($3, 1, 5) }
  ( lastts != curts ) { print "idx\t" curts "\t" NR; lastts = curts }
  END { print "prevlog_lines\t" NR }
  ' - >> $cachefile

    cat $logfile2 | awk '
  { curts = $1 "-" $2 "-" substr($3, 1, 5) }
  ( lastts != curts ) { print "idx\t" curts "\t" NR+'$(get_prevlog_lines_from_cache)'; lastts = curts }
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
    echo "logfile has changed: stored '$logfile1_stored_modtime', actual '$logfile1_cur_modtile'" 1>&2
    rm $cachefile
  fi

  if ! get_prevlog_lines_from_cache > /dev/null; then
    echo "broken cache file (no prevlog lines), deleting it" 1>&2
    rm $cachefile
  fi

  refresh_and_retry=0

  # First try to find it in cache without refreshing the cache

  # NOTE: as of now, it doesn't support a case when there were no messages
  # during whole minute at all. We just assume all our services do log
  # something at least once a minute.

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

awk_pattern=''
if [[ "$user_pattern" != "" ]]; then
  awk_pattern="!($user_pattern) {next}"
fi

lines_until_check=''
if [[ "$lines_until" != "" ]]; then
  lines_until_check="if (NR >= $lines_until) { next; }"
fi

awk_script='
BEGIN { curline=0; maxlines=` + strconv.Itoa(maxNumLines) + ` }
'$awk_pattern'
{
  stats[$1 "-" $2 "-" substr($3,1,5)]++;

  '$lines_until_check'

  lastlines[curline++] = $0;
  if (curline >= maxlines) {
    curline = 0;
  }

  next;
}

END {
  for (x in stats) {
    print "mstats:" x "," stats[x]
  }

  for (i = 0; i < maxlines; i++) {
    ln = curline + i;
    if (ln >= maxlines) {
      ln -= maxlines;
    }

    print "msg:" lastlines[ln];
  }
}
'
logfiles="$logfile1 $logfile2"

if [[ "$from_nr" != "" ]]; then
  # Let's see if we need to check the $logfile1 at all
  prevlog_lines=$(get_prevlog_lines_from_cache)
  if [[ $(( prevlog_lines < from_nr )) == 1 ]]; then
    echo "Ignoring prev log file" 1>&2
    from_nr=$(( from_nr - prevlog_lines ))
    if [[ "$to_nr" != "" ]]; then
      to_nr=$(( to_nr - prevlog_lines ))
    fi
    logfiles="$logfile2"
  fi
fi

if [[ "$from_nr" == "" && "$to_nr" == "" ]]; then
  cat $logfiles | awk "$awk_script" - | sort
elif [[ "$from_nr" != "" && "$to_nr" == "" ]]; then
  cat $logfiles | tail -n +$from_nr | awk "$awk_script" - | sort
elif [[ "$from_nr" == "" && "$to_nr" != "" ]]; then
  cat $logfiles | head -n $to_nr | awk "$awk_script" - | sort
else
  cat $logfiles | tail -n +$from_nr | head -n $((to_nr - from_nr)) | awk "$awk_script" - | sort
fi
`
