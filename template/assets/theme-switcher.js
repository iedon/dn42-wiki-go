let theme = window.localStorage.getItem("theme");

if (theme === null && window.matchMedia) {
    if (window.matchMedia("(prefers-color-scheme: light)").matches) {
        theme = "light";
    }
    
    window.matchMedia("(prefers-color-scheme: light)").addEventListener("change", e => {
        if (window.localStorage.getItem("theme") !== null) return;

        if (e.matches) {
            document.body.classList.add("light");
            theme = "light";
        } else {
            document.body.classList.remove("light");
            theme = "dark";
        }
    });
}

if (theme == "light") document.body.classList.add("light");

document.addEventListener("DOMContentLoaded", () => {
    const toggle = document.getElementById("toggle-theme");

    toggle.addEventListener("click", () => {
        document.body.classList.toggle("light");
    
        if (theme === "light") {
            window.localStorage.setItem("theme", "dark");
            theme = "dark";
        } else {
            window.localStorage.setItem("theme", "light");
            theme = "light";
        }
    });
});



