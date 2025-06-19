package com.vegusa.middleware.utils;

import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;

@Component
public class LogsUtils {

    public static void generateLog(String json, String action){
        DateTimeFormatter formatter = DateTimeFormatter.ofPattern("yyyy-MM-dd_HH-mm-ss");
        String timestamp = LocalDateTime.now().format(formatter);

        String fileName = "log_" + action + "_" + timestamp + ".json";
        Path path = Paths.get("logs", fileName);

        try {
            Files.createDirectories(path.getParent());

            Files.write(
                    path,
                    json.getBytes(),
                    StandardOpenOption.CREATE
            );

            System.out.println("Log guardado en: " + path.toAbsolutePath());

        } catch (IOException e) {
            e.printStackTrace();
        }
    }
}
