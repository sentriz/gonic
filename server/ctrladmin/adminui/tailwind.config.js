/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["*.tmpl", "**/*.tmpl"],
  theme: {
    screens: {
      sm: "100%",
      md: `870px`,
    },
    fontFamily: {
      mono: ["Inconsolata", "monospace"],
    },
  },
};
