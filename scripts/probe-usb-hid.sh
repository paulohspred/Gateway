#!/usr/bin/env bash
set -euo pipefail

shopt -s nullglob

read_text() {
  local path="$1"
  if [[ -r "$path" ]]; then
    tr -d '\n' < "$path" 2>/dev/null || true
  fi
}

found=0
for dev in /dev/hidraw*; do
  found=1
  name="$(basename "$dev")"
  sys="/sys/class/hidraw/$name/device"
  uevent="$sys/uevent"

  hid_id=""
  hid_name=""
  hid_uniq=""
  if [[ -r "$uevent" ]]; then
    hid_id="$(awk -F= '$1 == "HID_ID" {print $2}' "$uevent" || true)"
    hid_name="$(awk -F= '$1 == "HID_NAME" {sub(/^HID_NAME=/, ""); print}' "$uevent" || true)"
    hid_uniq="$(awk -F= '$1 == "HID_UNIQ" {sub(/^HID_UNIQ=/, ""); print}' "$uevent" || true)"
  fi

  hid_bus=""
  vendor_id=""
  product_id=""
  if [[ "$hid_id" == *:*:* ]]; then
    IFS=: read -r hid_bus vendor_raw product_raw <<< "$hid_id"
    vendor_id="${vendor_raw: -4}"
    product_id="${product_raw: -4}"
    vendor_id="${vendor_id,,}"
    product_id="${product_id,,}"
  fi

  resolved="$(readlink -f "$sys" 2>/dev/null || true)"
  cursor="$resolved"
  usb_vendor=""
  usb_product=""
  usb_manufacturer=""
  usb_product_name=""
  usb_serial=""
  usb_busnum=""
  usb_devnum=""
  interface_number=""

  while [[ -n "$cursor" && "$cursor" == /sys/* && "$cursor" != "/sys" && "$cursor" != "/" ]]; do
    if [[ -z "$interface_number" && -r "$cursor/bInterfaceNumber" ]]; then
      interface_number="$(read_text "$cursor/bInterfaceNumber")"
    fi
    if [[ -r "$cursor/idVendor" && -r "$cursor/idProduct" ]]; then
      usb_vendor="$(read_text "$cursor/idVendor")"
      usb_product="$(read_text "$cursor/idProduct")"
      usb_manufacturer="$(read_text "$cursor/manufacturer")"
      usb_product_name="$(read_text "$cursor/product")"
      usb_serial="$(read_text "$cursor/serial")"
      usb_busnum="$(read_text "$cursor/busnum")"
      usb_devnum="$(read_text "$cursor/devnum")"
      break
    fi
    parent="$(dirname "$cursor")"
    [[ "$parent" == "$cursor" ]] && break
    cursor="$parent"
  done

  descriptor="$sys/report_descriptor"
  descriptor_bytes=""
  descriptor_sha256=""
  if [[ -r "$descriptor" ]]; then
    descriptor_bytes="$(wc -c < "$descriptor" | tr -d ' ' || true)"
    if command -v sha256sum >/dev/null 2>&1; then
      descriptor_sha256="$(sha256sum "$descriptor" 2>/dev/null | awk '{print $1}' || true)"
    fi
  fi

  permissions="$(stat -Lc '%a:%U:%G' "$dev" 2>/dev/null || true)"

  printf 'device=%q hid_id=%q hid_bus=%q vendor_id=%q product_id=%q hid_name=%q hid_serial=%q usb_vendor=%q usb_product=%q usb_manufacturer=%q usb_product_name=%q usb_serial=%q usb_busnum=%q usb_devnum=%q interface=%q report_descriptor_bytes=%q report_descriptor_sha256=%q permissions=%q\n' \
    "$dev" "${hid_id:-unknown}" "${hid_bus:-unknown}" "${vendor_id:-unknown}" "${product_id:-unknown}" \
    "${hid_name:-unknown}" "${hid_uniq:-unknown}" "${usb_vendor:-unknown}" "${usb_product:-unknown}" \
    "${usb_manufacturer:-unknown}" "${usb_product_name:-unknown}" "${usb_serial:-unknown}" \
    "${usb_busnum:-unknown}" "${usb_devnum:-unknown}" "${interface_number:-unknown}" \
    "${descriptor_bytes:-unknown}" "${descriptor_sha256:-unknown}" "${permissions:-unknown}"
done

if [[ "$found" -eq 0 ]]; then
  echo "no /dev/hidraw* devices found"
fi
