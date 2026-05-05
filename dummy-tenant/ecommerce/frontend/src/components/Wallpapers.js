import React, { useEffect, useState } from "react";
import axios from "axios";

const Wallpapers = () => {
  const [wallpapers, setWallpapers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");

  useEffect(() => {
    axios
      .get("/api/v1/wallpaper/wallpapers")
      .then((res) => {
        setWallpapers(res.data);
        setLoading(false);
      })
      .catch((err) => {
        console.error("Error fetching wallpapers:", err);
        setLoading(false);
      });
  }, []);

  const addToCart = (wallpaper) => {
    const item = {
      product_name: wallpaper.name,
      quantity: 1,
      price: 499.0, // fixed price for demo
    };
    axios
      .post("/api/v1/ecommerce/add-cart", item)
      .then(() => setMessage(`${wallpaper.name} added to cart!`))
      .catch(() => setMessage("Error adding to cart!"));
  };

  if (loading) return <p className="text-center mt-10">Loading wallpapers...</p>;

  return (
    <div>
      {message && (
        <p className="text-green-600 font-medium text-center my-3">{message}</p>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
        {wallpapers.map((wp) => (
          <div
            key={wp.id}
            className="bg-white rounded-lg shadow-md overflow-hidden transition hover:shadow-lg"
          >
            <img
              src={wp.image_url}
              alt={wp.name}
              className="h-48 w-full object-cover"
            />
            <div className="p-4">
              <h3 className="text-lg font-semibold mb-2">{wp.name}</h3>
              <p className="text-gray-500 mb-3">Price: ₹499</p>
              <button
                onClick={() => addToCart(wp)}
                className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 transition"
              >
                Add to Cart
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default Wallpapers;
