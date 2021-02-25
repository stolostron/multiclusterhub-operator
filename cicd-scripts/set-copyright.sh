# Copyright Contributors to the Open Cluster Management project

#!/bin/bash

# TESTED ON MAC!

TMP_FILE="tmp_file"

ALL_FILES=$(find . -name "*")

COMMUNITY_COPY_HEADER_FILE="$PWD/cicd-scripts/copyright-header.txt"

RH_COPY_HEADER="Copyright (c) 2020 Red Hat, Inc."

COMMUNITY_COPY_HEADER_STRING=$(cat $COMMUNITY_COPY_HEADER_FILE)

NEWLINE="\n"

if [[ "$DRY_RUN" == true ]]; then
   echo "---- Beginning dry run ----"
fi

for FILE in $ALL_FILES
do
    echo "FILE: $FILE:"
    if [[ -d $FILE ]] ; then
        echo -e "\t-Directory; skipping"
        continue 
    fi

    COMMENT_START="# "
    COMMENT_END=""

    if [[ $FILE  == *".go" ]]; then
        COMMENT_START="// "
    fi

    if [[ $FILE  == *".ts" || $FILE  == *".tsx" ]]; then
        COMMENT_START="/* "
        COMMENT_END=" */"
    fi

    if [[ $FILE  == *".md" ]]; then
        COMMENT_START="[comment]: # ( "
        COMMENT_END=" )"
    fi

    if [[ $FILE  == *".html" ]]; then
        COMMENT_START="<!-- "
        COMMENT_END=" -->"
    fi

    if [[ $FILE  == *".go"       \
            || $FILE == *".yaml" \
            || $FILE == *".yml"  \
            || $FILE == *".sh"   \
            || $FILE == *"Dockerfile" \
            || $FILE == *"Makefile"  \
            || $FILE == *".gitignore"  \
            || $FILE == *".md"  ]]; then

        COMMUNITY_HEADER_AS_COMMENT="$COMMENT_START$COMMUNITY_COPY_HEADER_STRING$COMMENT_END"

        if grep -q "$COMMUNITY_HEADER_AS_COMMENT" "$FILE"; then
            echo "\t- Header already exists; skipping"
        else

            if [[ "$DRY_RUN" == true ]]; then
                echo -e "\t- [DRY RUN] Will add Community copyright header to file"
                continue
            fi    

            ALL_COPYRIGHTS=""

            RH_COPY_HEADER_AS_COMMENT="$COMMENT_START$RH_COPY_HEADER$COMMENT_END"

            if grep -q "$RH_COPY_HEADER_AS_COMMENT" "$FILE"; then
                ALL_COPYRIGHTS="$ALLCOPYRIGHTS$RH_COPY_HEADER_AS_COMMENT$NEWLINE"
                grep -v "$RH_COPY_HEADER_AS_COMMENT" $FILE > $TMP_FILE
                mv $TMP_FILE  $FILE
                echo -e "\t- Has Red Hat copyright header"
            fi
            
            ALL_COPYRIGHTS="$ALL_COPYRIGHTS$COMMUNITY_HEADER_AS_COMMENT$NEWLINE"
            echo -e $ALL_COPYRIGHTS > $TMP_FILE
            cat $FILE >> $TMP_FILE
            mv $TMP_FILE $FILE

            echo -e "\t- Adding Community copyright header to file"
        fi
    else
        echo -e "\t- DO NOTHING"
    fi
done

rm -f $TMP_FILE