for (const input of document.querySelectorAll("input[type=file].auto-submit") || []) {
  input.onchange = (e) => e.target.form.submit();
}
