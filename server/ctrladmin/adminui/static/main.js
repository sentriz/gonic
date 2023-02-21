for (const input of document.querySelectorAll("input.auto-submit, select.auto-submit") || []) {
  input.onchange = (e) => e.target.form.submit();
}
