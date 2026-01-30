const path = require("path");

module.exports = (env, argv) => ({
  mode: argv.mode || "development",

  entry: "./src/main.ts",

  output: {
    path: path.resolve(__dirname, "dist"),
    filename: "app.js",
  },

  module: {
    rules: [
      {
        test: /\.ts$/,
        use: "ts-loader",
        exclude: /node_modules/,
      },
    ],
  },

  resolve: {
    extensions: [".ts", ".js"],
  },

  devtool: argv.mode === "production" ? false : "source-map",
});
