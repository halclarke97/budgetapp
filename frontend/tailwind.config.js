/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Space Grotesk"', '"Manrope"', 'system-ui', 'sans-serif'],
      },
      colors: {
        ink: '#101826',
        sky: '#e8f3ff',
      },
      boxShadow: {
        card: '0 15px 35px rgba(16, 24, 38, 0.08)',
      },
    },
  },
  plugins: [],
}
