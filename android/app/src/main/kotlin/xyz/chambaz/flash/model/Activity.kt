package xyz.chambaz.flash.model

import com.google.gson.annotations.SerializedName

data class DayActivity(
    @SerializedName("date") val date: String,
    @SerializedName("done") val done: Int,
    @SerializedName("due") val due: Int,
) {
    val ratio: Float get() = if (due == 0) 0f else (done.toFloat() / due).coerceIn(0f, 1f)
}
