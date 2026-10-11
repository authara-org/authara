const path = require("path");

module.exports = (env, argv) => ({
  mode: argv.mode || "development",

  entry: {
    app: "./src/main.ts",
    "microsoft-redirect": "./src/microsoftRedirect.ts",
  },

  output: {
    path: path.resolve(__dirname, "dist"),
    filename: "[name].js",
    chunkFilename: "[id].app.[contenthash:16].js",
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
