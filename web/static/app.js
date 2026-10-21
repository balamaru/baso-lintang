document.documentElement.classList.add("js");
const menu = document.getElementById("menu-button");
const navigation = document.getElementById("navigation");
function closeMenu() { navigation.classList.remove("open"); menu.setAttribute("aria-expanded", "false"); }
menu.addEventListener("click", () => { const open = navigation.classList.toggle("open"); menu.setAttribute("aria-expanded", String(open)); });
navigation.addEventListener("click", event => { if (event.target.closest("a")) closeMenu(); });
document.addEventListener("keydown", event => { if (event.key === "Escape" && navigation.classList.contains("open")) { closeMenu(); menu.focus(); } });
