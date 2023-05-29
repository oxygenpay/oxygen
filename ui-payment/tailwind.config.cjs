/** @type {import("tailwindcss").Config} */
module.exports = {
    content: ["./src/**/*.{html,js,ts,tsx}", "./node_modules/tw-elements/dist/js/**/*.js"],
    theme: {
        extend: {
            colors: {
                primary: "#f0fc7f",
                "primary-darker": "#d5e33f",
                card: {
                    desc: "#8A9495",
                    error: "#D63650"
                },
                main: {
                    "green-1": "#50AF95",
                    "green-2": "#49D1AC",
                    "green-3": "#D0E6E8",
                    "error": "#D63650", 
                    "red-1": "#fff2f0",
                    "red-2": "#ffccc7"
                }
            },
            maxWidth: {
                "xl-desc-size": "300px",
                "sm-desc-size": "209px"
            },
            minHeight: {
                "mobile-card": "600px"
            },
            height: {
                "mobile-card-height": "calc(100vh - 3.75rem)"
            }
        },
        screens: {
            "xs": {"max": "390px"},
            "sm": {"max": "639px"},
            "md": {"min": "390px", "max": "639px"},
            "lg": "640px"
        }
    },
    plugins: [require("tw-elements/dist/plugin")]
};
