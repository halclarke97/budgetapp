/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./index.html', './src/**/*.{js,jsx,ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#1f2937',
        sky: '#0ea5e9',
        mint: '#10b981',
        sand: '#f5f5f4'
      }
    }
  },
  plugins: []
}
