for (const form of document.querySelectorAll("form.file-upload") || []) {
  const input = form.querySelector("input[type=file]");
  if (!input) continue;
  input.onchange = (e) => form.submit();
}
