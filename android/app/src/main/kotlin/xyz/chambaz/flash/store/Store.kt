package xyz.chambaz.flash.store

import android.content.Context
import com.google.gson.Gson
import com.google.gson.reflect.TypeToken
import xyz.chambaz.flash.model.DeckItem

class Store(context: Context) {
    private val prefs = context.getSharedPreferences("flash", Context.MODE_PRIVATE)
    private val gson = Gson()

    fun loadUrl(): String = prefs.getString("url", "") ?: ""
    fun loadToken(): String = prefs.getString("token", "") ?: ""
    fun loadTheme(): String = prefs.getString("theme", "black") ?: "black"
    fun loadAccentIndex(): Int = prefs.getInt("accent_index", 6)
    fun loadThreshold(): Float = prefs.getFloat("threshold", 0.7f)
    fun loadStep(): Long = prefs.getLong("pace_ms", 604_800_000L)

    fun saveUrl(v: String) = prefs.edit().putString("url", v).apply()
    fun saveToken(v: String) = prefs.edit().putString("token", v).apply()
    fun saveTheme(v: String) = prefs.edit().putString("theme", v).apply()
    fun saveAccentIndex(v: Int) = prefs.edit().putInt("accent_index", v).apply()
    fun saveThreshold(v: Float) = prefs.edit().putFloat("threshold", v).apply()
    fun saveStep(v: Long) = prefs.edit().putLong("pace_ms", v).apply()

    fun loadCachedDecks(): List<DeckItem> {
        val json = prefs.getString("decks_json", null) ?: return emptyList()
        val type = object : TypeToken<List<DeckItem>>() {}.type
        return gson.fromJson(json, type) ?: emptyList()
    }

    fun saveDecks(decks: List<DeckItem>) =
        prefs.edit().putString("decks_json", gson.toJson(decks)).apply()

    fun hasConfig(): Boolean = loadUrl().isNotEmpty() && loadToken().isNotEmpty()

    fun baseUrl(): String = loadUrl().trimEnd('/')
}
