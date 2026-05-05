import React, { useState } from "react";
import Wallpapers from "./components/Wallpapers";
import Cart from "./components/Cart";

function App() {
  const [view, setView] = useState("wallpapers"); // 'wallpapers' or 'cart'

  return (
    <div className="min-h-screen flex flex-col items-center">
      <header className="w-full bg-blue-600 text-white py-4 text-center text-2xl font-semibold shadow-md">
        Golang E-commerce & Wallpaper App
      </header>

      <nav className="mt-4 flex gap-4">
        <button
          className={`px-4 py-2 rounded ${
            view === "wallpapers"
              ? "bg-blue-600 text-white"
              : "bg-gray-200 text-gray-700"
          }`}
          onClick={() => setView("wallpapers")}
        >
          Wallpapers
        </button>
        <button
          className={`px-4 py-2 rounded ${
            view === "cart"
              ? "bg-blue-600 text-white"
              : "bg-gray-200 text-gray-700"
          }`}
          onClick={() => setView("cart")}
        >
          Cart
        </button>
      </nav>

      <main className="flex-1 w-full max-w-5xl mt-6 px-4">
        {view === "wallpapers" ? <Wallpapers /> : <Cart />}
      </main>

      <footer className="w-full bg-gray-800 text-gray-100 py-3 text-center text-sm mt-auto">
        © {new Date().getFullYear()} Golang Project | Vivek’s Full Integration Demo
      </footer>
    </div>
  );
}

export default App;
