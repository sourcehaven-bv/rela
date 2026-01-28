#!/bin/bash
# Remove duplicate consecutive screen dumps

DUMP_DIR="screendumps"

if [ ! -d "$DUMP_DIR" ]; then
    echo "Error: Directory '$DUMP_DIR' does not exist"
    exit 1
fi

echo "Deduplicating screen dumps in $DUMP_DIR/"
echo ""

# Get sorted list of screen dump files
files=($(ls -1 "$DUMP_DIR"/screen_*.txt 2>/dev/null | sort))

if [ ${#files[@]} -eq 0 ]; then
    echo "No screen dump files found"
    exit 0
fi

echo "Found ${#files[@]} total screen dumps"

removed_count=0
kept_count=1
prev_file="${files[0]}"

echo "Keeping: $(basename "$prev_file")"

# Compare each file with the previous one
for ((i=1; i<${#files[@]}; i++)); do
    current_file="${files[$i]}"

    # Compare files using cmp (faster than diff for binary comparison)
    if cmp -s "$prev_file" "$current_file"; then
        # Files are identical - remove the duplicate
        rm "$current_file"
        echo "Removed: $(basename "$current_file") (duplicate of $(basename "$prev_file"))"
        ((removed_count++))
    else
        # Files are different - keep it and make it the new reference
        echo "Keeping: $(basename "$current_file")"
        prev_file="$current_file"
        ((kept_count++))
    fi
done

echo ""
echo "Summary:"
echo "  Total files processed: ${#files[@]}"
echo "  Unique files kept: $kept_count"
echo "  Duplicates removed: $removed_count"

# Optionally renumber remaining files
read -p "Renumber remaining files sequentially? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo "Renumbering files..."

    # Create temp directory
    temp_dir="${DUMP_DIR}_temp"
    mkdir -p "$temp_dir"

    # Get remaining files in order
    remaining_files=($(ls -1 "$DUMP_DIR"/screen_*.txt 2>/dev/null | sort))

    # Copy to temp with new numbering
    counter=1
    for file in "${remaining_files[@]}"; do
        new_name=$(printf "screen_%04d.txt" "$counter")
        cp "$file" "$temp_dir/$new_name"
        ((counter++))
    done

    # Replace original directory
    rm -f "$DUMP_DIR"/screen_*.txt
    mv "$temp_dir"/* "$DUMP_DIR/"
    rmdir "$temp_dir"

    echo "Files renumbered: 1 to $kept_count"
fi

echo ""
echo "Done!"
