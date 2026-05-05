import React, { useEffect, useState } from "react";
import axios from "axios";

const Cart = () => {
  const [items, setItems] = useState([]);
  const [message, setMessage] = useState("");

  const fetchCart = () => {
    axios
      .get("/api/v1/ecommerce/view-cart")
      .then((res) => setItems(res.data))
      .catch((err) => console.error(err));
  };

  useEffect(() => {
    fetchCart();
  }, []);

  const updateQuantity = (name, change, unitPrice) => {
    axios
      .post("/api/v1/ecommerce/update-cart", {
        product_name: name,
        change,
        unit_price: unitPrice,
      })
      .then(() => {
        fetchCart();
        setMessage("Cart updated!");
      })
      .catch(() => setMessage("Error updating cart!"));
  };

  const checkout = () => {
    axios
      .post("/api/v1/ecommerce/checkout")
      .then(() => {
        setItems([]);
        setMessage("Checkout complete!");
      })
      .catch(() => setMessage("Checkout failed!"));
  };

  return (
    <div>
      {message && (
        <p className="text-green-600 text-center font-medium my-3">{message}</p>
      )}
      {items.length === 0 ? (
        <p className="text-center mt-10 text-gray-500">
          Your cart is empty. Add wallpapers!
        </p>
      ) : (
        <div className="bg-white rounded-lg shadow-md p-4">
          <table className="w-full border-collapse">
            <thead>
              <tr className="bg-gray-100">
                <th className="p-2 text-left">Product</th>
                <th className="p-2 text-center">Quantity</th>
                <th className="p-2 text-center">Price (₹)</th>
                <th className="p-2 text-center">Actions</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr key={it.id} className="border-t">
                  <td className="p-2">{it.product_name}</td>
                  <td className="p-2 text-center">{it.quantity}</td>
                  <td className="p-2 text-center">{it.price.toFixed(2)}</td>
                  <td className="p-2 text-center">
                    <button
                      className="px-2 py-1 bg-blue-500 text-white rounded mr-2"
                      onClick={() =>
                        updateQuantity(it.product_name, 1, it.price / it.quantity)
                      }
                    >
                      +
                    </button>
                    <button
                      className="px-2 py-1 bg-red-500 text-white rounded"
                      onClick={() =>
                        updateQuantity(it.product_name, -1, it.price / it.quantity)
                      }
                    >
                      −
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="text-right mt-4">
            <button
              onClick={checkout}
              className="bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700"
            >
              Checkout
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

export default Cart;
