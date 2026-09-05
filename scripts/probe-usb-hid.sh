#!/usr/bin/env bash
set -euo pipefail

shopt -s nullglob
found=0
for dev in /dev/hidraw*; do
  found=1
  name="$(basename "$dev")"
  uevent="/sys/class/hidraw/$name/device/uevent"
  hid_id=""
  hid_name=""
  hid_uniq=""
  if [[ -r "$uevent" ]]; then
    hid_id="$(awk -F= '$1 == "HID_ID" {print $2}' "$uevent" || true)"
    hid_name="$(awk -F= '$1 == "HID_NAME" {sub(/^HID_NAME=/, ""); print}' "$uevent" || true)"
    hid_uniq="$(awk -F= '$1 == "HID_UNIQ" {sub(/^HID_UNIQ=/, ""); print}' "$uevent" || true)"
  fi
  printf '%s\n' "device=$dev hid_id=${hid_id:-unknown} name=${hid_name:-unknown} serial=${hid_uniq:-unknown}"
done

if [[ "$found" -eq 0 ]]; then
  echo "no /dev/hidraw* devices found"
fi
